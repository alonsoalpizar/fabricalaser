
PRE-FLIGHT CHECKLIST
&
PLAN DE IMPLEMENTACIÓN
WhatsApp Business API (Cloud API) — FabricaLaser.com
Arquitectura: Número Prepago Dedicado + Escalamiento a Asesor Humano
Fecha: __________ | Versión: 2.0 | Autor: Alonso Alpízar
1. Contexto y Lección Aprendida
Este documento nace de la experiencia directa de un bloqueo de cuenta Meta Developer durante un intento previo de pase a producción de la app de WhatsApp Business API para FabricaLaser. El bloqueo resultó en pérdida temporal de acceso a la cuenta developer, necesidad de procesos paralelos de recuperación (Admin Lockout Recovery + verificación de negocio), y desvinculación preventiva del número WhatsApp Business del Business Account.
La cuenta ha sido recuperada exitosamente. Este documento establece la estrategia actualizada para el pase a producción, incorporando una arquitectura de número prepago dedicado que aísla el riesgo operativo.
2. Arquitectura: Número Prepago Dedicado
2.1 Decisión Arquitectónica
En lugar de usar el número principal de FabricaLaser para el WABA (WhatsApp Business Account), se utilizará un número prepago dedicado exclusivamente para la integración con el Cloud API. Esto separa la capa de comunicación pública (lo que ve el cliente) de los recursos internos del negocio.
2.2 Ventajas de esta Arquitectura
–	Aislamiento de riesgo: si Meta bloquea el WABA o el número, la operación diaria de FabricaLaser NO se ve afectada
–	Libertad para experimentar con la API, templates, y flujos de bot sin riesgo sobre el número principal
–	Posibilidad de cambiar de proveedor, reconfigurar o migrar sin afectar la comunicación del negocio
–	Costo mínimo: un chip Kölbi prepago cuesta menos de ₡1,000 y solo necesita recargas periódicas para mantenerse activo
–	No requiere Coexistence: separación completa de responsabilidades entre API y teléfono
–	Escalabilidad: si en el futuro se necesitan más líneas WABA, se agregan más chips prepago
2.3 Flujo de Comunicación
El flujo operativo quedaría estructurado de la siguiente manera:
1.	Cliente contacta: El cliente escribe al número WABA (prepago). Este número es el que se publica en el sitio web, redes sociales y Google Business Profile.
2.	Agente automático atiende: El webhook en hooks.fabricalaser.com recibe el mensaje. El agente (Gemini + function calling) procesa la consulta: cotizaciones por dimensiones, catálogo de materiales, información de servicios.
3.	Escalamiento a humano: Cuando el agente detecta que necesita intervención humana (consulta compleja, cierre de venta, problema técnico), se notifica al asesor. Opciones de notificación: alerta interna vía Telegram, email, o notificación push.
4.	Asesor responde: El asesor humano responde al cliente a través del mismo número WABA (vía panel de administración o directamente por la API). El cliente siempre ve un solo número consistente.
2.4 Diagrama de Componentes

CAPA PÚBLICA
Número Prepago WABA
(+506 XXXX-XXXX)
Lo que ve el cliente	CAPA DE PROCESAMIENTO
hooks.fabricalaser.com
handler.go / sender.go / processor.go
Gemini + Redis + PostgreSQL	CAPA INTERNA
Número real del negocio
(teléfono físico)
Protegido, nunca expuesto

3. Causas Comunes de Bloqueo o Rechazo
3.1 Razones típicas de rechazo en App Review
1.	Permisos excesivos: Solicitar permisos que la app no necesita realmente. Solo pedir whatsapp_business_management y whatsapp_business_messaging.
2.	Screencast insuficiente: Meta requiere un video demostrativo del flujo real de la app. Un video vago o sin contexto causa rechazo inmediato.
3.	Política de privacidad incompleta: La URL debe estar pública, sin login, y mencionar explícitamente el uso de datos de WhatsApp/Meta.
4.	Descripción de uso vaga: Cada permiso necesita justificación detallada. Frases genéricas como 'para comunicarse con clientes' no son suficientes.
5.	Verificación de negocio incompleta: El Business Manager debe tener la verificación aprobada ANTES de solicitar el review.
6.	Display Name no aprobado: Debe coincidir con la marca en el sitio web y documentos legales.
7.	App en modo incorrecto: La app debe ser tipo 'Business' en Meta for Developers.
3.2 Causas de bloqueo de cuenta
Un bloqueo de cuenta developer puede ocurrir por: actividad sospechosa durante configuración, múltiples intentos fallidos de verificación, cambios rápidos en el Business Account, o inconsistencias entre Business Manager y documentos de verificación. Evitar hacer múltiples cambios en poco tiempo.
 
4. Pre-flight Checklist
Cada ítem debe estar en estado 'Listo' antes de iniciar el proceso de App Review.

4.1 Número Prepago Dedicado
Categoría	Requisito / Acción	Estado
SIM	Chip prepago adquirido (Kölbi, Movistar o Claro)	Pendiente
SIM	Número activo y capaz de recibir SMS y llamadas (para OTP)	Pendiente
SIM	Número NO registrado previamente en ningún WhatsApp	Pendiente
SIM	Recarga mínima realizada para mantener línea activa	Pendiente
SIM	Número documentado en registro interno de FabricaLaser	Pendiente
SIM	Plan de recargas periódicas definido (cada 30-60 días)	Pendiente

4.2 Verificación de Negocio (Meta Business Manager)
Categoría	Requisito / Acción	Estado
Identidad	Verificación de negocio aprobada en Business Manager (Security Center)	Pendiente
Identidad	Documentos legales subidos (cédula jurídica, patente, o equivalente CR)	Pendiente
Identidad	Nombre del negocio coincide en: Business Manager, sitio web y documentos	Pendiente
Identidad	Dirección física registrada y coincidente	Pendiente
Dominio	Dominio verificado en Business Manager (fabricalaser.com)	Pendiente
Dominio	Sitio web activo y con contenido coherente con el negocio	Pendiente

4.3 Configuración de la App (Meta for Developers)
Categoría	Requisito / Acción	Estado
App	App creada como tipo 'Business' en Meta for Developers	Pendiente
App	WhatsApp agregado como producto en la app	Pendiente
App	Información básica completa (nombre, icono, descripción)	Pendiente
App	URL de política de privacidad configurada y accesible públicamente	Pendiente
App	URL de Términos de Servicio configurada	Pendiente
Permisos	whatsapp_business_management solicitado	Pendiente
Permisos	whatsapp_business_messaging solicitado	Pendiente
Permisos	NO se solicitan permisos adicionales innecesarios	Pendiente

4.4 Configuración de WhatsApp
Categoría	Requisito / Acción	Estado
Número	Número prepago registrado y verificado (OTP recibido)	Pendiente
Número	Display Name 'FabricaLaser' aprobado por Meta	Pendiente
Número	Display Name coincide con marca visible en fabricalaser.com	Pendiente
Webhook	URL del webhook configurada (hooks.fabricalaser.com)	Pendiente
Webhook	Webhook verificado y respondiendo al challenge	Pendiente
Webhook	Suscripción a eventos: messages, message_status	Pendiente
Token	System User creado en Business Manager	Pendiente
Token	Token permanente generado (NO usar temporal en prod)	Pendiente
Token	Token almacenado seguro (env vars, no en código fuente)	Pendiente
Templates	Al menos un template creado y aprobado	Pendiente
Templates	Templates siguen guías de Meta (sin mayúsculas excesivas, contenido claro)	Pendiente

4.5 Privacidad y Cumplimiento
Categoría	Requisito / Acción	Estado
Privacidad	Política de privacidad publicada en fabricalaser.com	Pendiente
Privacidad	Menciona explícitamente: recolección de datos vía WhatsApp	Pendiente
Privacidad	Menciona: tipo de datos (mensajes, número, nombre)	Pendiente
Privacidad	Menciona: propósito del procesamiento	Pendiente
Privacidad	Menciona: retención y eliminación de datos	Pendiente
Opt-in	Mecanismo de opt-in documentado	Pendiente
Opt-in	Opt-in explícito para WhatsApp (no reutilizar SMS/email)	Pendiente
Compliance	Agente NO permite conversaciones abiertas tipo IA general	Pendiente
Compliance	Cada interacción tiene propósito de negocio claro	Pendiente

4.6 Preparación del App Review
Categoría	Requisito / Acción	Estado
Screencast	Video grabado mostrando flujo completo del agente	Pendiente
Screencast	Video muestra: cliente envía mensaje → bot responde → cotización	Pendiente
Screencast	Video muestra: escalamiento a humano	Pendiente
Screencast	Video explica cada elemento de UI visible	Pendiente
Screencast	Duración: 2-5 minutos, claro y conciso	Pendiente
Docs	Descripción detallada por cada permiso	Pendiente
Docs	Justificación técnica: por qué cada permiso es necesario	Pendiente
Docs	Credenciales de prueba proporcionadas	Pendiente
Docs	Webhook accesible sin bloqueos de IP/geo	Pendiente
 
5. Plan de Implementación por Fases
Plan semanal con entregables concretos. Los tiempos asumen que la cuenta Meta Developer ya está recuperada y estabilizada.

Sem.	Fase	Acciones Clave	Entregable
1	Preparación	Adquirir chip prepago Kölbi/Movistar/Claro. Verificar que recibe SMS. Publicar política de privacidad y TOS en fabricalaser.com. Verificar dominio en Business Manager. Completar verificación de negocio si no está lista.	SIM activa. Privacidad publicada. Dominio verificado.
2	App & WABA	Crear/verificar app tipo Business. Agregar producto WhatsApp. Registrar número prepago (OTP). Solicitar aprobación de Display Name 'FabricaLaser'. Crear System User y token permanente.	App configurada. Número registrado. Token generado.
3	Webhook & Bot	Configurar webhook con número prepago. Actualizar handler.go/sender.go con nuevo Phone Number ID y token. Crear templates de mensaje. Probar flujo completo con números de test.	Webhook operativo. Templates aprobados. Flujo probado.
4	Review	Grabar screencast del flujo. Preparar descripciones de permisos. Enviar App Review. Monitorear estado.	App Review enviado.
5	Go-Live	Post-aprobación: activar en producción. Publicar número WABA en sitio web, redes y Google Business. Configurar alertas de escalamiento a humano (Telegram). Monitorear quality rating.	Servicio en producción.

6. Diseño del Escalamiento a Humano
El mecanismo de escalamiento es clave para el cumplimiento de Meta 2026 (no se permite IA de propósito general) y para la calidad del servicio.
6.1 Triggers de Escalamiento
–	El cliente solicita explícitamente hablar con un humano
–	La consulta excede las capacidades del agente (proyectos complejos, negociación de precios)
–	Se detecta insatisfacción o frustración en el tono del mensaje
–	La cotización supera un monto umbral definido
–	El agente ha respondido N veces sin resolver la consulta
6.2 Canal de Notificación Interna
Cuando se activa un escalamiento, el sistema notifica al asesor humano. Se recomienda usar Telegram como canal interno de notificación dado que la integración ya está planificada como canal alternativo. El flujo sería:
–	Agente detecta trigger de escalamiento
–	Sistema envía alerta a grupo/bot de Telegram interno con: nombre del cliente, resumen de conversación, tipo de consulta
–	Asesor humano revisa y responde al cliente a través del WABA (vía API o panel)
–	El cliente siempre ve un solo número y una experiencia continua
6.3 Respuesta al Cliente Durante Escalamiento
El agente debe enviar un mensaje claro al cliente indicando que un asesor se comunicará pronto, dando un tiempo estimado realista. Ejemplo: 'Un asesor de FabricaLaser revisará tu consulta y te responderá en los próximos minutos.' Esto cumple con las expectativas de Meta de que el bot tenga comportamiento predecible.
7. Cumplimiento Meta 2026
7.1 Restricciones sobre Bots de IA
Desde enero 2026, Meta prohibió chatbots de IA de propósito general en la WhatsApp Business API. Solo se permiten flujos de automatización con resultados predecibles y propósito de negocio: bots de soporte, cotización, seguimiento, agendamiento.
Impacto para FabricaLaser: El agente ya está orientado a funciones específicas (cotización por dimensiones, catálogo de materiales, escalamiento). Es compatible, pero hay que verificar que el prompt de sistema del agente Gemini NO permita conversaciones abiertas fuera del contexto de negocio.
7.2 Pricing Actualizado (julio 2025)
Meta cambió a facturación por mensaje entregado. Categorías: Marketing, Utility, Authentication, Service. Mensajes de servicio dentro de la ventana de 24h son gratuitos. Cada WABA recibe 1,000 conversaciones de servicio gratuitas por mes. Las respuestas del agente a mensajes de clientes caen en la categoría Service (gratuita si es dentro de 24h).
8. Nota sobre Coexistencia (Referencia Futura)
Aunque la arquitectura actual usa un número prepago dedicado (sin necesidad de Coexistence), esta funcionalidad queda documentada como opción futura si se decide unificar el número principal con la API.
Costa Rica (+506) está soportado para Coexistence (no aparece en la lista de exclusiones). Los únicos países excluidos actualmente son Nigeria y Sudáfrica. Requisitos: WhatsApp Business App v2.24.17+, Facebook Page vinculada, Admin en Business Manager. Limitaciones: broadcasts deshabilitados en app, grupos no sincronizan, throughput de 5 msg/seg, app debe abrirse cada 14 días.
Nota importante: Si el número fue previamente desvinculado de un WABA, se debe esperar 1-2 meses antes de poder usarlo en Coexistence. Durante ese tiempo el número debe estar activo en uso regular.
9. Plan de Contingencia
1.	No entrar en pánico. Los rechazos de App Review permiten reenvío con correcciones. Un rechazo NO bloquea la cuenta.
2.	Documentar el error exacto. Capturar screenshots del mensaje de rechazo o bloqueo.
3.	No hacer cambios rápidos. Cambios múltiples en poco tiempo disparan bloqueos preventivos.
4.	Número es fungible. El número prepago es reemplazable. Comprar otro chip y reconfigurar.
5.	Telegram como fallback. La integración de Telegram ya está planificada como canal alternativo de atención.
6.	Contactar soporte Meta. Usar Business Help Center y Admin Lockout Recovery si es necesario.
10. Referencias
–	Meta for Developers — WhatsApp Cloud API: developers.facebook.com/docs/whatsapp/
–	App Review: developers.facebook.com/docs/resp-plat-initiatives/individual-processes/app-review
–	WhatsApp Business Policy: business.whatsapp.com/policy
–	Coexistence Docs: developers.facebook.com/documentation/business-messaging/whatsapp/embedded-signup/onboarding-business-app-users/
–	Permissions Reference: developers.facebook.com/docs/permissions/
–	WhatsApp Compliance 2026: Políticas actualizadas enero 2026
