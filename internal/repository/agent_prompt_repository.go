package repository

import (
	"errors"
	"fmt"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

// ErrAgentPromptNotFound se retorna cuando el agent_key no existe.
var ErrAgentPromptNotFound = errors.New("agent prompt not found")

// ErrVersionNotFound se retorna cuando se pide rollback o versión específica que no existe.
var ErrVersionNotFound = errors.New("agent prompt version not found")

type AgentPromptRepository struct {
	db *gorm.DB
}

func NewAgentPromptRepository() *AgentPromptRepository {
	return &AgentPromptRepository{db: database.Get()}
}

// FindAll retorna todos los prompts activos ordenados por id (consistente con orden de seed).
func (r *AgentPromptRepository) FindAll() ([]models.AgentPrompt, error) {
	var prompts []models.AgentPrompt
	if err := r.db.Where("is_active = ?", true).Order("id ASC").Find(&prompts).Error; err != nil {
		return nil, err
	}
	return prompts, nil
}

// FindByKey busca un prompt por agent_key. Retorna ErrAgentPromptNotFound si no existe.
// IMPORTANTE: puede retornar una fila con Body="" (válido en la ventana entre migración
// y primer Seed). El caller (Provider.Get) debe chequear len(Body)==0 para activar
// fallback en vez de servir string vacío al LLM.
func (r *AgentPromptRepository) FindByKey(key string) (*models.AgentPrompt, error) {
	var p models.AgentPrompt
	err := r.db.Where("agent_key = ? AND is_active = ?", key, true).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentPromptNotFound
		}
		return nil, err
	}
	return &p, nil
}

// FindVersions retorna las últimas `limit` versiones de un prompt ordenadas descendente.
// Si limit <= 0, retorna todas.
func (r *AgentPromptRepository) FindVersions(promptID uint, limit int) ([]models.AgentPromptVersion, error) {
	var versions []models.AgentPromptVersion
	q := r.db.Where("agent_prompt_id = ?", promptID).Order("version DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// FindVersion retorna una versión específica. Error si no existe.
func (r *AgentPromptRepository) FindVersion(promptID uint, version int) (*models.AgentPromptVersion, error) {
	var v models.AgentPromptVersion
	err := r.db.Where("agent_prompt_id = ? AND version = ?", promptID, version).First(&v).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVersionNotFound
		}
		return nil, err
	}
	return &v, nil
}

// SaveNewVersion atómicamente:
//  1. Inserta una fila en agent_prompt_versions con el body ACTUAL de agent_prompts
//     (para preservar qué había antes del cambio)
//  2. Update agent_prompts: body=newBody, version++, updated_by=userID
//
// Si la transacción falla, ni el histórico ni el prompt activo se tocan (atomicidad).
// Retorna la nueva versión creada (>= 2; los prompts seedeados arrancan en v1).
func (r *AgentPromptRepository) SaveNewVersion(promptID uint, newBody string, userID uint, note *string) (int, error) {
	var newVersion int
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Cargar fila actual (lock para evitar race condition con otro save simultáneo)
		var current models.AgentPrompt
		if err := tx.Clauses().Where("id = ?", promptID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentPromptNotFound
			}
			return err
		}

		// Guardar el body actual como versión histórica (version actual)
		// Solo si body no vacío — no guardamos el vacío inicial del seed como histórico
		if current.Body != "" {
			historical := models.AgentPromptVersion{
				AgentPromptID: current.ID,
				Version:       current.Version,
				Body:          current.Body,
				UpdatedBy:     current.UpdatedBy,
				Note:          nil, // la nota del save anterior ya está en el fila del save previo
			}
			if err := tx.Create(&historical).Error; err != nil {
				return fmt.Errorf("save historical version: %w", err)
			}
		}

		// Actualizar prompt activo con nueva versión
		newVersion = current.Version + 1
		uid := userID
		if err := tx.Model(&current).Updates(map[string]any{
			"body":       newBody,
			"version":    newVersion,
			"updated_by": &uid,
		}).Error; err != nil {
			return fmt.Errorf("update active prompt: %w", err)
		}

		// Crear versión nueva (la que queda activa) con su nota
		// Esto es para que el historial incluya la versión ACTUAL también — permite diff
		// entre actual y cualquier anterior sin tratamiento especial
		active := models.AgentPromptVersion{
			AgentPromptID: current.ID,
			Version:       newVersion,
			Body:          newBody,
			UpdatedBy:     &uid,
			Note:          note,
		}
		if err := tx.Create(&active).Error; err != nil {
			return fmt.Errorf("save active version as history: %w", err)
		}

		return nil
	})

	if err != nil {
		return 0, err
	}
	return newVersion, nil
}

// RollbackResult es el retorno enriquecido del rollback para que la UI muestre
// contexto claro al gestor (v6 ↩ Restaurado desde v1).
type RollbackResult struct {
	NewVersion          int
	RestoredFromVersion int
	BodyPreview         string // primeros 200 chars del body restaurado
}

// Rollback restablece el body del prompt a una versión anterior, creando una NUEVA
// versión activa (no borra el historial).
//
// Proceso:
//  1. Busca body de la versión target
//  2. SaveNewVersion con ese body, note="Restaurado desde v{N}"
//  3. Marca la fila nueva con restored_from_version=N
//  4. Retorna RollbackResult con los datos para la UI
func (r *AgentPromptRepository) Rollback(promptID uint, toVersion int, userID uint) (*RollbackResult, error) {
	// Fetch fuera de transacción — si no existe ya sabemos antes de lockear
	target, err := r.FindVersion(promptID, toVersion)
	if err != nil {
		return nil, err
	}

	note := fmt.Sprintf("Restaurado desde v%d", toVersion)
	newVer, err := r.SaveNewVersion(promptID, target.Body, userID, &note)
	if err != nil {
		return nil, err
	}

	// Después del SaveNewVersion, la última fila creada en agent_prompt_versions tiene
	// version=newVer. Le actualizamos restored_from_version para marcarlo como rollback.
	if err := r.db.Model(&models.AgentPromptVersion{}).
		Where("agent_prompt_id = ? AND version = ?", promptID, newVer).
		Update("restored_from_version", toVersion).Error; err != nil {
		return nil, fmt.Errorf("mark rollback origin: %w", err)
	}

	preview := target.Body
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	return &RollbackResult{
		NewVersion:          newVer,
		RestoredFromVersion: toVersion,
		BodyPreview:         preview,
	}, nil
}

// SeedBody llena el body de un prompt SOLO si actualmente está vacío.
// Usado por prompts.Provider.Seed() en el primer arranque para poblar los prompts
// con los fallbacks hardcoded. Idempotente: no sobrescribe ediciones del gestor.
// Retorna true si hubo escritura, false si ya tenía body.
func (r *AgentPromptRepository) SeedBody(agentKey, body string) (bool, error) {
	result := r.db.Model(&models.AgentPrompt{}).
		Where("agent_key = ? AND (body IS NULL OR body = '')", agentKey).
		Update("body", body)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
