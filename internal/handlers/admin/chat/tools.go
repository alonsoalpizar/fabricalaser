package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/vertexai/genai"
	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/services/pricing"
)

const (
	consultarBlankURL = "http://localhost:8083/api/v1/blanks/consultar"
	httpToolTimeout   = 10 * time.Second
)

var cedulaRegex = regexp.MustCompile(`^\d{9,10}$`)

// toolExecutor agrupa las dependencias compartidas que necesitan los handlers
// de las 6 tools del chat admin (R1: usa pricing.Calculator directo, no HTTP).
type toolExecutor struct {
	calculator    *pricing.Calculator
	configLoader  *pricing.ConfigLoader
	userRepo      *repository.UserRepository
	quoteRepo     *repository.QuoteRepository
	materialRepo  *repository.MaterialRepository
	techRepo      *repository.TechnologyRepository
	httpClient    *http.Client
	internalToken string
}

func newToolExecutor() *toolExecutor {
	configLoader := pricing.NewConfigLoader(database.Get())
	return &toolExecutor{
		calculator:    pricing.NewCalculator(configLoader),
		configLoader:  configLoader,
		userRepo:      repository.NewUserRepository(),
		quoteRepo:     repository.NewQuoteRepository(),
		materialRepo:  repository.NewMaterialRepository(),
		techRepo:      repository.NewTechnologyRepository(),
		httpClient:    &http.Client{Timeout: httpToolTimeout},
		internalToken: os.Getenv("INTERNAL_API_TOKEN"),
	}
}

// ─── Function Declarations ───────────────────────────────────────────────────

func toolDeclarations() []*genai.FunctionDeclaration {
	return []*genai.FunctionDeclaration{
		calcularCotizacionTool(),
		consultarBlankTool(),
		listarMaterialesTool(),
		listarTecnologiasTool(),
		buscarClienteTool(),
		historialCotizacionesTool(),
	}
}

func calcularCotizacionTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name: "calcular_cotizacion",
		Description: "Calcula precio detallado de un trabajo de grabado o corte láser usando el motor de pricing oficial. " +
			"Devuelve breakdown completo: tiempos, costos, factores aplicados, ambos modelos de precio (híbrido y por valor) y cuál ganó. " +
			"Usar cuando el gestor ya proporcionó material, medidas y cantidad.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"alto_cm":           {Type: genai.TypeNumber, Description: "Alto del área a grabar/cortar en centímetros"},
				"ancho_cm":          {Type: genai.TypeNumber, Description: "Ancho del área a grabar/cortar en centímetros"},
				"cantidad":          {Type: genai.TypeInteger, Description: "Número de unidades a producir"},
				"technology_id":     {Type: genai.TypeInteger, Description: "ID de tecnología láser (ver listar_tecnologias o IDs en system prompt)"},
				"material_id":       {Type: genai.TypeInteger, Description: "ID del material (ver listar_materiales o IDs en system prompt)"},
				"engrave_type_id":   {Type: genai.TypeInteger, Description: "1=Vectorial, 2=Rasterizado, 3=Fotograbado, 4=3D/Relieve. Default: 1"},
				"thickness":         {Type: genai.TypeNumber, Description: "Grosor del material en mm. Default: 3.0"},
				"material_included": {Type: genai.TypeBoolean, Description: "true si FabricaLaser provee el material, false si el cliente lo trae"},
				"incluye_corte":     {Type: genai.TypeBoolean, Description: "true si el trabajo incluye corte del perímetro además del grabado"},
				"cut_technology_id": {Type: genai.TypeInteger, Description: "ID de tecnología para el corte cuando es diferente. Solo en Caso 3B (UV graba + CO2 corta)"},
			},
			Required: []string{"alto_cm", "ancho_cm", "cantidad", "technology_id", "material_id", "material_included", "incluye_corte"},
		},
	}
}

func consultarBlankTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "consultar_blank",
		Description: "Consulta precio y disponibilidad de un blank (producto preconfigurado: llaveros, medallas, etc.). Usar cuando el gestor pregunte por estos productos del catálogo.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"categoria": {Type: genai.TypeString, Description: "Categoría del blank: 'llavero', 'medalla', etc."},
				"cantidad":  {Type: genai.TypeInteger, Description: "Cantidad de unidades que el cliente quiere"},
				"blank_id":  {Type: genai.TypeInteger, Description: "ID específico del blank. 0 (o no incluir) si no se conoce — el tool retorna todas las opciones de la categoría"},
			},
			Required: []string{"categoria", "cantidad"},
		},
	}
}

func listarMaterialesTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "listar_materiales",
		Description: "Lista los materiales disponibles en el catálogo, opcionalmente filtrando por cortabilidad o categoría. Útil cuando el gestor pregunta '¿qué materiales tenemos?' o '¿qué cortamos?'.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"cortable_solo": {Type: genai.TypeBoolean, Description: "true para listar solo materiales que se pueden cortar con CO2"},
				"categoria":     {Type: genai.TypeString, Description: "Filtrar por categoría (ej: 'madera', 'acrilico', 'metal'). Vacío = todos"},
			},
		},
	}
}

func listarTecnologiasTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "listar_tecnologias",
		Description: "Lista las tecnologías láser disponibles (CO2, UV, Fibra, MOPA) con sus IDs. Útil cuando el gestor pregunta '¿qué tecnologías tenemos?'.",
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: map[string]*genai.Schema{},
		},
	}
}

func buscarClienteTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "buscar_cliente",
		Description: "Busca un cliente en la base de datos por cédula (9 o 10 dígitos) o por nombre/email (búsqueda fuzzy). Devuelve hasta 20 resultados con datos de contacto.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"query": {Type: genai.TypeString, Description: "Cédula numérica (9-10 dígitos) o nombre/email para buscar"},
			},
			Required: []string{"query"},
		},
	}
}

func historialCotizacionesTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "historial_cotizaciones",
		Description: "Devuelve las cotizaciones recientes de un cliente (por user_id obtenido con buscar_cliente). Incluye tecnología, material, precio final y fecha.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"user_id": {Type: genai.TypeInteger, Description: "ID del usuario (obtener primero con buscar_cliente)"},
				"limit":   {Type: genai.TypeInteger, Description: "Máximo de cotizaciones a retornar. Default 10, máximo 50"},
			},
			Required: []string{"user_id"},
		},
	}
}

// ─── Dispatcher ──────────────────────────────────────────────────────────────

// executeFunction despacha la llamada a la implementación correcta.
// adminID se pasa por parámetro para futuras decisiones de auditoría/permisos por gestor
// (hoy no se usa pero queda en la firma para no romper en v2).
//
// Aplica normalización JSON al resultado antes de devolverlo: el SDK de Vertex AI
// (genai.FunctionResponse) requiere tipos compatibles con structpb, y NO acepta
// []map[string]any. El round-trip JSON convierte todo a map[string]any + []any.
func (e *toolExecutor) executeFunction(ctx context.Context, adminID uint, fc *genai.FunctionCall) (map[string]any, error) {
	_ = adminID
	var result map[string]any
	var err error

	switch fc.Name {
	case "calcular_cotizacion":
		result, err = e.execCalcularCotizacion(ctx, fc.Args)
	case "consultar_blank":
		result, err = e.execConsultarBlank(ctx, fc.Args)
	case "listar_materiales":
		result, err = e.execListarMateriales(ctx, fc.Args)
	case "listar_tecnologias":
		result, err = e.execListarTecnologias(ctx, fc.Args)
	case "buscar_cliente":
		result, err = e.execBuscarCliente(ctx, fc.Args)
	case "historial_cotizaciones":
		result, err = e.execHistorialCotizaciones(ctx, fc.Args)
	default:
		return nil, fmt.Errorf("tool desconocida: %s", fc.Name)
	}

	if err != nil {
		return nil, err
	}
	return normalizeForStructPB(result), nil
}

// normalizeForStructPB hace round-trip JSON para que cualquier slice/map
// concretos ([]map[string]any, []SomeStruct, []clienteOut, etc.) se conviertan
// a tipos plain ([]any, map[string]any) que structpb sí acepta.
func normalizeForStructPB(v map[string]any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"error": "no se pudo serializar resultado: " + err.Error()}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"error": "no se pudo deserializar resultado: " + err.Error()}
	}
	return out
}

// ─── Implementaciones ────────────────────────────────────────────────────────

func (e *toolExecutor) execCalcularCotizacion(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	altoCM := getNumber(args, "alto_cm")
	anchoCM := getNumber(args, "ancho_cm")
	cantidad := getInt(args, "cantidad")
	techID := uint(getInt(args, "technology_id"))
	materialID := uint(getInt(args, "material_id"))
	engraveTypeID := uint(getInt(args, "engrave_type_id"))
	thickness := getNumber(args, "thickness")
	materialIncluded := getBool(args, "material_included")
	incluyeCorte := getBool(args, "incluye_corte")

	var cutTechID *uint
	if v := getInt(args, "cut_technology_id"); v > 0 {
		u := uint(v)
		cutTechID = &u
	}

	if altoCM <= 0 || anchoCM <= 0 {
		return map[string]any{"error": "Las medidas deben ser mayores a 0"}, nil
	}
	if altoCM > 100 || anchoCM > 100 {
		return map[string]any{"error": "Medidas fuera del rango de trabajo (máx 100cm)"}, nil
	}
	if techID == 0 || materialID == 0 {
		return map[string]any{"error": "technology_id y material_id son requeridos"}, nil
	}
	if cantidad < 1 {
		cantidad = 1
	}
	if thickness <= 0 {
		thickness = 3.0
	}
	if engraveTypeID == 0 && !incluyeCorte {
		engraveTypeID = 1
	}

	analysis := pricing.BuildSyntheticAnalysis(altoCM*10, anchoCM*10, incluyeCorte, engraveTypeID)

	pr, err := e.calculator.Calculate(
		analysis, techID, materialID, engraveTypeID,
		thickness, cantidad, materialIncluded, cutTechID, false,
	)
	if err != nil {
		return map[string]any{"error": "Error calculando precio: " + err.Error()}, nil
	}

	priceFinal := math.Max(pr.PriceHybridTotal, pr.PriceValueTotal)
	priceUnit := math.Round(priceFinal / float64(cantidad))

	// Resolver nombres de tech y material para que la explicación sea legible
	techName, materialName := "", ""
	if cfg, err := e.configLoader.Load(); err == nil {
		if t := cfg.GetTechnology(techID); t != nil {
			techName = t.Name
		}
		if m := cfg.GetMaterial(materialID); m != nil {
			materialName = m.Name
		}
	}

	resp := map[string]any{
		"precio_total":      math.Round(priceFinal),
		"precio_unitario":   priceUnit,
		"cantidad":          cantidad,
		"area_cm2":          altoCM * anchoCM,
		"tecnologia":        techName,
		"material":          materialName,
		"tiempo_total_min":  pr.TimeTotalMins,
		"tiempo_grabado_min": pr.TimeEngraveMins,
		"tiempo_corte_min":  pr.TimeCutMins,
		"tiempo_setup_min":  pr.TimeSetupMins,
		"costo_base":        pr.CostBase,
		"costo_grabado":     pr.CostEngrave,
		"costo_corte":       pr.CostCut,
		"costo_setup":       pr.CostSetup,
		"costo_material":    pr.CostMaterialWithWaste,
		"material_incluido": materialIncluded,
		"factor_material":   pr.FactorMaterial,
		"factor_grabado":    pr.FactorEngrave,
		"factor_uv_premium": pr.FactorUVPremium,
		"factor_margen":     pr.FactorMargin,
		"descuento_volumen_pct": pr.DiscountVolumePct,
		"price_hybrid_unit":  pr.PriceHybridUnit,
		"price_hybrid_total": pr.PriceHybridTotal,
		"price_value_unit":   pr.PriceValueUnit,
		"price_value_total":  pr.PriceValueTotal,
		"price_model_ganador": pr.PriceModel,
		"price_model_detalle": pr.PriceModelDetail,
		"status":             string(pr.Status),
		"complexity_note":    pr.ComplexityNote,
	}

	if pr.UsedFallbackSpeeds {
		resp["fallback_warning"] = pr.FallbackWarning
	}
	if cutTechID != nil {
		resp["cut_technology_id"] = *cutTechID
	}

	return resp, nil
}

func (e *toolExecutor) execConsultarBlank(ctx context.Context, args map[string]any) (map[string]any, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank marshal: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, httpToolTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, consultarBlankURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.internalToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.internalToken)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank read: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("execConsultarBlank unmarshal: %w", err)
	}

	slog.Info("admin_chat: consultar_blank ejecutado",
		"categoria", args["categoria"],
		"cantidad", args["cantidad"],
		"encontrado", result["encontrado"],
	)

	return result, nil
}

func (e *toolExecutor) execListarMateriales(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	cortableSolo := getBool(args, "cortable_solo")
	categoria := strings.ToLower(strings.TrimSpace(getString(args, "categoria")))

	all, err := e.materialRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("execListarMateriales: %w", err)
	}

	out := make([]map[string]any, 0, len(all))
	for _, m := range all {
		if !m.IsActive {
			continue
		}
		if cortableSolo && !m.IsCuttable {
			continue
		}
		if categoria != "" && !strings.Contains(strings.ToLower(m.Category), categoria) {
			continue
		}

		notas := ""
		if m.Notes != nil {
			notas = *m.Notes
		}

		out = append(out, map[string]any{
			"id":          m.ID,
			"nombre":      m.Name,
			"categoria":   m.Category,
			"factor":      m.Factor,
			"cuttable":    m.IsCuttable,
			"thicknesses": json.RawMessage(m.Thicknesses),
			"notas":       notas,
		})
	}

	return map[string]any{"total": len(out), "materiales": out}, nil
}

func (e *toolExecutor) execListarTecnologias(ctx context.Context, args map[string]any) (map[string]any, error) {
	_, _ = ctx, args

	all, err := e.techRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("execListarTecnologias: %w", err)
	}

	out := make([]map[string]any, 0, len(all))
	for _, t := range all {
		if !t.IsActive {
			continue
		}
		descripcion := ""
		if t.Description != nil {
			descripcion = *t.Description
		}
		out = append(out, map[string]any{
			"id":                 t.ID,
			"code":               t.Code,
			"nombre":             t.Name,
			"uv_premium_factor":  t.UVPremiumFactor,
			"descripcion":        descripcion,
		})
	}

	return map[string]any{"total": len(out), "tecnologias": out}, nil
}

type clienteOut struct {
	ID                uint   `json:"id"`
	Cedula            string `json:"cedula"`
	Nombre            string `json:"nombre"`
	Apellido          string `json:"apellido,omitempty"`
	Email             string `json:"email"`
	Telefono          string `json:"telefono,omitempty"`
	Provincia         string `json:"provincia,omitempty"`
	Canton            string `json:"canton,omitempty"`
	QuoteQuota        int    `json:"quote_quota"`
	QuotesUsed        int    `json:"quotes_used"`
	TotalCotizaciones int    `json:"total_cotizaciones"`
	Activo            bool   `json:"activo"`
}

func toClienteOut(u *models.User, totalCotizaciones int) clienteOut {
	out := clienteOut{
		ID:                u.ID,
		Cedula:            u.Cedula,
		Nombre:            u.Nombre,
		Email:             u.Email,
		QuoteQuota:        u.QuoteQuota,
		QuotesUsed:        u.QuotesUsed,
		TotalCotizaciones: totalCotizaciones,
		Activo:            u.Activo,
	}
	if u.Apellido != nil {
		out.Apellido = *u.Apellido
	}
	if u.Telefono != nil {
		out.Telefono = *u.Telefono
	}
	if u.Provincia != nil {
		out.Provincia = *u.Provincia
	}
	if u.Canton != nil {
		out.Canton = *u.Canton
	}
	return out
}

func (e *toolExecutor) execBuscarCliente(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	query := strings.TrimSpace(getString(args, "query"))
	if query == "" {
		return map[string]any{"encontrados": 0, "clientes": []clienteOut{}}, nil
	}

	// Cédula numérica → búsqueda exacta
	if cedulaRegex.MatchString(query) {
		u, err := e.userRepo.FindByCedula(query)
		if err != nil {
			return map[string]any{"encontrados": 0, "clientes": []clienteOut{}}, nil
		}
		count, _ := e.quoteRepo.CountByUserThisMonth(u.ID)
		return map[string]any{
			"encontrados": 1,
			"clientes":    []clienteOut{toClienteOut(u, int(count))},
		}, nil
	}

	// Búsqueda fuzzy por nombre/email (UserRepository.ListAll usa ILIKE)
	users, _, err := e.userRepo.ListAll(20, 0, query, "", nil)
	if err != nil {
		return nil, fmt.Errorf("execBuscarCliente: %w", err)
	}

	out := make([]clienteOut, 0, len(users))
	for i := range users {
		count, _ := e.quoteRepo.CountByUserThisMonth(users[i].ID)
		out = append(out, toClienteOut(&users[i], int(count)))
	}

	return map[string]any{"encontrados": len(out), "clientes": out}, nil
}

func (e *toolExecutor) execHistorialCotizaciones(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	userID := uint(getInt(args, "user_id"))
	limit := getInt(args, "limit")
	if userID == 0 {
		return map[string]any{"error": "user_id es requerido"}, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	quotes, err := e.quoteRepo.FindByUserIDWithRelations(userID, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("execHistorialCotizaciones: %w", err)
	}

	out := make([]map[string]any, 0, len(quotes))
	for _, q := range quotes {
		entry := map[string]any{
			"id":            q.ID,
			"created_at":    q.CreatedAt.Format(time.RFC3339),
			"cantidad":      q.Quantity,
			"thickness":     q.Thickness,
			"price_final":   q.PriceFinal,
			"price_model":   q.PriceModel,
			"status":        string(q.Status),
		}
		if q.Technology != nil {
			entry["tecnologia"] = q.Technology.Name
		}
		if q.Material != nil {
			entry["material"] = q.Material.Name
		}
		if q.EngraveType != nil {
			entry["tipo_grabado"] = q.EngraveType.Name
		}
		out = append(out, entry)
	}

	return map[string]any{"total": len(out), "cotizaciones": out}, nil
}

// ─── Helpers de extracción de args ───────────────────────────────────────────

func getNumber(args map[string]any, key string) float64 {
	v, ok := args[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func getInt(args map[string]any, key string) int {
	return int(getNumber(args, key))
}

func getBool(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func getString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
