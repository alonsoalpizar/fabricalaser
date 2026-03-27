package email

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"net"
	"net/smtp"
)

const (
	smtpAddr  = "localhost:25"
	fromAddr  = "noreply@fabricalaser.com"
	fromName  = "FabricaLaser"
	siteURL   = "https://fabricalaser.com"
	cotizarURL = "https://fabricalaser.com/cotizar"
)

// SendWelcome envía el correo de bienvenida a un cliente recién registrado.
// Se ejecuta en goroutine — nunca bloquea el registro.
func SendWelcome(toEmail, nombre string) {
	go func() {
		if err := sendWelcome(toEmail, nombre); err != nil {
			log.Printf("[email] Error enviando bienvenida a %s: %v", toEmail, err)
		} else {
			log.Printf("[email] Bienvenida enviada a %s", toEmail)
		}
	}()
}

func sendWelcome(toEmail, nombre string) error {
	subject := "¡Bienvenido a FabricaLaser!"
	body := buildWelcomeBody(toEmail, nombre)
	return sendMail(toEmail, subject, body)
}

func buildWelcomeBody(toEmail, nombre string) string {
	var buf bytes.Buffer

	buf.WriteString(`<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { margin: 0; padding: 0; background-color: #f4f4f4; font-family: Arial, sans-serif; }
    .wrap { max-width: 620px; margin: 32px auto; background: #ffffff; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 12px rgba(0,0,0,0.1); }
    .header { background: #9B2020; padding: 36px 32px; text-align: center; }
    .header img { height: 48px; }
    .header h1 { color: #ffffff; margin: 16px 0 0; font-size: 22px; letter-spacing: 0.5px; }
    .body { padding: 32px; color: #1a1a1a; font-size: 15px; line-height: 1.7; }
    .body p { margin: 0 0 16px; }
    .highlight { background: #fff7ed; border-left: 4px solid #9B2020; padding: 14px 18px; border-radius: 4px; margin: 20px 0; font-size: 14px; color: #7c2d12; }
    .btn { display: inline-block; background: #9B2020; color: #ffffff; text-decoration: none; padding: 13px 32px; border-radius: 6px; font-size: 15px; font-weight: bold; margin: 8px 0; }
    .features { background: #f9fafb; border-radius: 8px; padding: 20px 24px; margin: 20px 0; }
    .features ul { margin: 0; padding: 0 0 0 20px; color: #374151; font-size: 14px; }
    .features ul li { margin-bottom: 8px; }
    .footer { background: #1a1a1a; color: #9ca3af; font-size: 12px; text-align: center; padding: 20px 32px; line-height: 1.6; }
    .footer a { color: #f87171; text-decoration: none; }
  </style>
</head>
<body>
<div class="wrap">

  <div class="header">
    <h1>¡Bienvenido a FabricaLaser!</h1>
  </div>

  <div class="body">
    <p>Hola <strong>`)

	buf.WriteString(html.EscapeString(nombre))

	buf.WriteString(`</strong>,</p>

    <p>Tu cuenta ha sido creada exitosamente. Ahora tienes acceso al cotizador en línea de FabricaLaser, la plataforma de corte y grabado láser de precisión en Costa Rica.</p>

    <div class="highlight">
      Tu cuenta incluye <strong>5 cotizaciones gratuitas</strong> para que puedas explorar nuestros servicios. ¡Aprovéchalas!
    </div>

    <div class="features">
      <strong>¿Qué puedes hacer?</strong>
      <ul>
        <li>Subir archivos SVG y obtener cotizaciones instantáneas</li>
        <li>Elegir entre múltiples tecnologías: CO₂, UV, Fibra, MOPA</li>
        <li>Seleccionar materiales: madera, acrílico, cuero, metal y más</li>
        <li>Ver el historial de todas tus cotizaciones</li>
      </ul>
    </div>

    <p style="text-align: center; margin-top: 28px;">
      <a href="`)
	buf.WriteString(cotizarURL)
	buf.WriteString(`" class="btn">Comenzar a cotizar</a>
    </p>

    <p style="font-size: 13px; color: #6b7280; margin-top: 24px;">
      Si tienes alguna consulta, escríbenos a <a href="mailto:info@fabricalaser.com" style="color:#9B2020;">info@fabricalaser.com</a> o visita <a href="`)
	buf.WriteString(siteURL)
	buf.WriteString(`" style="color:#9B2020;">fabricalaser.com</a>.
    </p>
  </div>

  <div class="footer">
    © FabricaLaser · Costa Rica<br>
    <a href="`)
	buf.WriteString(siteURL)
	buf.WriteString(`">fabricalaser.com</a>
  </div>

</div>
</body>
</html>`)

	return buf.String()
}

// SendPasswordReset envía el correo de recuperación de contraseña.
// Se ejecuta en goroutine — nunca bloquea el handler.
func SendPasswordReset(toEmail, nombre, token string) {
	go func() {
		if err := sendPasswordReset(toEmail, nombre, token); err != nil {
			log.Printf("[email] Error enviando reset a %s: %v", toEmail, err)
		} else {
			log.Printf("[email] Reset password enviado a %s", toEmail)
		}
	}()
}

func sendPasswordReset(toEmail, nombre, token string) error {
	subject := "Recuperación de contraseña — FabricaLaser"
	body := buildPasswordResetBody(toEmail, nombre, token)
	return sendMail(toEmail, subject, body)
}

func buildPasswordResetBody(toEmail, nombre, token string) string {
	resetLink := siteURL + "/reset-password?token=" + token
	var buf bytes.Buffer

	buf.WriteString(`<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { margin: 0; padding: 0; background-color: #f4f4f4; font-family: Arial, sans-serif; }
    .wrap { max-width: 620px; margin: 32px auto; background: #ffffff; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 12px rgba(0,0,0,0.1); }
    .header { background: #9B2020; padding: 36px 32px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 22px; letter-spacing: 0.5px; }
    .body { padding: 32px; color: #1a1a1a; font-size: 15px; line-height: 1.7; }
    .body p { margin: 0 0 16px; }
    .highlight { background: #fff7ed; border-left: 4px solid #9B2020; padding: 14px 18px; border-radius: 4px; margin: 20px 0; font-size: 14px; color: #7c2d12; }
    .btn { display: inline-block; background: #9B2020; color: #ffffff; text-decoration: none; padding: 13px 32px; border-radius: 6px; font-size: 15px; font-weight: bold; margin: 8px 0; }
    .footer { background: #1a1a1a; color: #9ca3af; font-size: 12px; text-align: center; padding: 20px 32px; line-height: 1.6; }
    .footer a { color: #f87171; text-decoration: none; }
    .link-text { word-break: break-all; font-size: 12px; color: #6b7280; }
  </style>
</head>
<body>
<div class="wrap">

  <div class="header">
    <h1>Recuperación de contraseña</h1>
  </div>

  <div class="body">
    <p>Hola <strong>`)

	buf.WriteString(html.EscapeString(nombre))

	buf.WriteString(`</strong>,</p>

    <p>Recibimos una solicitud para restablecer la contraseña de tu cuenta en FabricaLaser. Si no fuiste vos, ignorá este correo y tu cuenta seguirá igual.</p>

    <div class="highlight">
      Este enlace es válido por <strong>1 hora</strong> y solo puede usarse una vez.
    </div>

    <p style="text-align: center; margin-top: 28px;">
      <a href="`)
	buf.WriteString(resetLink)
	buf.WriteString(`" class="btn">Restablecer contraseña</a>
    </p>

    <p style="font-size: 13px; color: #6b7280; margin-top: 24px;">
      Si el botón no funciona, copiá este enlace en tu navegador:<br>
      <span class="link-text">`)
	buf.WriteString(html.EscapeString(resetLink))
	buf.WriteString(`</span>
    </p>

    <p style="font-size: 13px; color: #6b7280;">
      ¿Tenés alguna consulta? Escribinos a <a href="mailto:info@fabricalaser.com" style="color:#9B2020;">info@fabricalaser.com</a>
    </p>
  </div>

  <div class="footer">
    © FabricaLaser · Costa Rica<br>
    <a href="`)
	buf.WriteString(siteURL)
	buf.WriteString(`">fabricalaser.com</a>
  </div>

</div>
</body>
</html>`)

	return buf.String()
}

func sendMail(toEmail, subject, htmlBody string) error {
	conn, err := net.Dial("tcp", smtpAddr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := client.Mail(fromAddr); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(toEmail); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}

	mime := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n"
	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\n%s\r\n%s",
		fromName, fromAddr, toEmail, subject, mime, htmlBody)

	if _, err := fmt.Fprint(w, msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return w.Close()
}
