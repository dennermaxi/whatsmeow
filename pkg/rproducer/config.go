// Package rproducer contém modificações RProducer isoladas do upstream.
//
// **Princípio:** TODA mudança RProducer mora aqui (ou em comentários `[RPRODUCER]`
// no upstream). O upstream segue intacto na maior parte — só interceptamos no
// ponto único `sendToQueueOrWebhook` de `pkg/whatsmeow/service/whatsmeow.go`.
//
// **Por que isso?** Facilita rebase com upstream — quase nenhum conflito.
//
// **Tudo controlado por env var, default = comportamento upstream.** Pode usar
// imagem `rproducer-wa` sem mudar nada e ela se comporta como o Evolution Go
// puro. Pra ativar features RPRODUCER, definir env vars.
package rproducer

import (
	"os"
	"strings"
)

// envOn retorna true se a env var está setada com valor "truthy"
// (1, true, on, yes, enabled — case insensitive).
func envOn(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch v {
	case "1", "true", "on", "yes", "enabled":
		return true
	}
	return false
}

// PublishToQueueOverride retorna o nome da fila pra onde TODOS os eventos
// devem ser publicados (em vez de `instanceId.eventType` default).
//
// Configurar via env: `RPRODUCER_PUBLISH_TO_QUEUE=rproducer-messages.upsert`
//
// Usado pra paridade com o bridge Node.js que sempre publicava na fila
// `rproducer-messages.upsert`. Quando setado, TODA `Produce()` vai pra essa fila.
//
// String vazia = sem override, mantém comportamento upstream.
func PublishToQueueOverride() string {
	return strings.TrimSpace(os.Getenv("RPRODUCER_PUBLISH_TO_QUEUE"))
}

// EnvelopeFormat retorna "rproducer-v1" se devemos reformatar o postMap pra
// envelope compatível com o RProducerAgent (que espera `{event, instance, data,
// timestamp, source}` em vez do default `{event, instanceId, instanceToken,
// instanceName, data}`).
//
// Configurar via env: `RPRODUCER_ENVELOPE=rproducer-v1`
//
// String vazia = sem override.
func EnvelopeFormat() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("RPRODUCER_ENVELOPE")))
}

// FilterMessageOnly retorna true se devemos descartar TODOS os eventos exceto
// `Message` na hora de publicar. Usado pra reduzir tráfego no RabbitMQ — o
// bridge fazia esse filtro também.
//
// Configurar via env: `RPRODUCER_FILTER_MESSAGE_ONLY=true`
//
// Eventos como `SendMessage`, `ReadReceipt`, `Connection`, `Presence` etc
// continuam sendo processados internamente, só não são publicados externos.
func FilterMessageOnly() bool {
	return envOn("RPRODUCER_FILTER_MESSAGE_ONLY")
}

// DedupEnabled retorna true se devemos deduplicar mensagens por msg_id
// (LRU cache TTL 60s, max 50k entries). Evolution às vezes emite a mesma
// mensagem 2x (formato @lid + @s.whatsapp.net) — o dedup descarta o 2º.
//
// Configurar via env: `RPRODUCER_DEDUP=true`
func DedupEnabled() bool {
	return envOn("RPRODUCER_DEDUP")
}

// AnyEnabled retorna true se qualquer feature RPRODUCER está ativada.
// Usado pra short-circuit do interceptor (zero overhead se desligado).
func AnyEnabled() bool {
	return PublishToQueueOverride() != "" ||
		EnvelopeFormat() != "" ||
		FilterMessageOnly() ||
		DedupEnabled()
}
