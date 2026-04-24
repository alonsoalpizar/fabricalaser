package chat

import (
	"fmt"
	"strings"
)

// NOTA: el body del system prompt "admin_chat" (antes `const systemPromptAdmin`)
// ya no vive acá. Ahora se sirve desde internal/agent/prompts.Provider con:
//   - Fuente primaria: tabla agent_prompts (agent_key="admin_chat"), editable
//     desde el admin UI con hot reload vía pub/sub Redis.
//   - Fallback hardcoded: internal/agent/prompts/fallbacks.go (copia exacta
//     del texto original que vivía en este archivo).
//
// El adapter obtiene el body con promptProvider.Get("admin_chat") y lo
// concatena con buildAdminContextBlock(...) + el contexto dinámico de DB.

// buildAdminContextBlock arma el bloque DATOS DEL GESTOR específico de la sesión.
// Se concatena al system prompt base (servido por prompts.Provider) antes del
// contexto dinámico de DB.
func buildAdminContextBlock(adminID uint, adminName string) string {
	var b strings.Builder
	b.WriteString("\n\n## DATOS DEL GESTOR (sesión actual)\n")
	b.WriteString(fmt.Sprintf("- ID interno: %d\n", adminID))
	if adminName != "" {
		b.WriteString(fmt.Sprintf("- Nombre: %s\n", adminName))
	}
	b.WriteString("- Rol: admin (acceso total)\n")
	return b.String()
}
