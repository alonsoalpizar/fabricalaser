package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ImageDownloader descarga imágenes desde la Media API de Meta.
type ImageDownloader struct {
	accessToken string
	apiVersion  string
	httpClient  *http.Client
}

// NewImageDownloader construye el downloader leyendo credenciales de variables de entorno.
func NewImageDownloader() *ImageDownloader {
	version := os.Getenv("WHATSAPP_API_VERSION")
	if version == "" {
		version = "v22.0"
	}
	return &ImageDownloader{
		accessToken: os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		apiVersion:  version,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// DownloadImage obtiene la URL del media desde la Graph API y descarga los bytes de la imagen.
// Retorna (imageBytes, mimeType, error).
func (d *ImageDownloader) DownloadImage(ctx context.Context, mediaID string) ([]byte, string, error) {
	// Paso 1: obtener metadata del media (URL + mime_type)
	metaURL := fmt.Sprintf("https://graph.facebook.com/%s/%s", d.apiVersion, mediaID)
	metaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("imageDownloader: error creando request metadata: %w", err)
	}
	metaReq.Header.Set("Authorization", "Bearer "+d.accessToken)

	metaResp, err := d.httpClient.Do(metaReq)
	if err != nil {
		return nil, "", fmt.Errorf("imageDownloader: error obteniendo metadata: %w", err)
	}
	defer metaResp.Body.Close()

	if metaResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(metaResp.Body)
		return nil, "", fmt.Errorf("imageDownloader: Meta respondió %d al pedir metadata: %s", metaResp.StatusCode, string(body))
	}

	var mediaInfo struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
	}
	if err := json.NewDecoder(metaResp.Body).Decode(&mediaInfo); err != nil {
		return nil, "", fmt.Errorf("imageDownloader: error decodificando metadata: %w", err)
	}
	if mediaInfo.URL == "" {
		return nil, "", fmt.Errorf("imageDownloader: URL vacía en metadata de media %s", mediaID)
	}

	// Paso 2: descargar los bytes de la imagen desde la URL de CDN de Meta
	imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaInfo.URL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("imageDownloader: error creando request de imagen: %w", err)
	}
	imgReq.Header.Set("Authorization", "Bearer "+d.accessToken)

	imgResp, err := d.httpClient.Do(imgReq)
	if err != nil {
		return nil, "", fmt.Errorf("imageDownloader: error descargando imagen: %w", err)
	}
	defer imgResp.Body.Close()

	if imgResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(imgResp.Body)
		return nil, "", fmt.Errorf("imageDownloader: error al descargar imagen %d: %s", imgResp.StatusCode, string(body))
	}

	imageBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("imageDownloader: error leyendo bytes de imagen: %w", err)
	}

	mimeType := mediaInfo.MimeType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	return imageBytes, mimeType, nil
}
