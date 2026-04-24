package llm

import (
	"context"
	"fmt"
)

// stubClient es un Client fallback que retorna error descriptivo en cada llamada.
// Se usa cuando el cliente inicial del factory no pudo construirse (ej. credenciales
// inválidas, proveedor temporalmente inaccesible). Permite que el servicio arranque
// y el gestor pueda corregir la configuración desde /admin/configuracion-llm.html
// sin necesidad de acceso SSH.
type stubClient struct {
	provider string
	initErr  error
}

func newStubClient(provider string, initErr error) Client {
	return &stubClient{provider: provider, initErr: initErr}
}

func (s *stubClient) Chat(ctx context.Context, system string, history []Message, tools []ToolDef, opts ...ChatOption) (*Response, error) {
	return nil, fmt.Errorf("llm no configurado: cliente %q falló al inicializar (%v). Configurar desde /admin/configuracion-llm", s.provider, s.initErr)
}

func (s *stubClient) TestConnection(ctx context.Context) error {
	return fmt.Errorf("llm stub: %w", s.initErr)
}

func (s *stubClient) SupportsImages() bool { return false }

func (s *stubClient) Provider() string { return s.provider + "-stub" }
