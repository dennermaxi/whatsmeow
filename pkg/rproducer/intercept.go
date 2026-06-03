package rproducer

import (
	"encoding/json"
	"time"
)

// InterceptResult — resultado da intercepção de um evento antes de ser publicado.
type InterceptResult struct {
	// ShouldSend — se false, o evento é descartado silenciosamente (não publica).
	ShouldSend bool
	// QueueName — fila pra publicar. Se PublishToQueueOverride() setada, vira ela.
	// Senão, mantém o queueName original.
	QueueName string
	// JSONData — payload final (pode ter sido reformatado pelo envelope override).
	JSONData []byte
}

// Intercept — ponto único de injeção de modificações RProducer pré-publish.
//
// Aplicado em `sendToQueueOrWebhook` (whatsmeow.go) e nos publishes globais.
// Se nenhuma feature RPRODUCER tá ativada, retorna passthrough (shouldSend=true,
// queueName/jsonData inalterados) com overhead próximo de zero.
//
// Ordem das operações (importante: deduplicate ANTES de reformatar pra economizar):
//  1. Filter (FILTER_MESSAGE_ONLY) — descarta non-Message
//  2. Dedup (DEDUP) — descarta msg_id já vista nos últimos 60s
//  3. Envelope (ENVELOPE=rproducer-v1) — reformata pro Agent
//  4. Queue override (PUBLISH_TO_QUEUE) — troca fila destino
func Intercept(queueName string, jsonData []byte) InterceptResult {
	if !AnyEnabled() {
		return InterceptResult{ShouldSend: true, QueueName: queueName, JSONData: jsonData}
	}

	// Deserializa postMap pra inspecionar event/msg_id
	var postMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &postMap); err != nil {
		// Não conseguiu decodificar — passa direto (não rejeita por safety)
		return InterceptResult{ShouldSend: true, QueueName: queueName, JSONData: jsonData}
	}

	eventType, _ := postMap["event"].(string)

	// 1) FILTER_MESSAGE_ONLY — só passa eventos "Message"
	if FilterMessageOnly() && eventType != "Message" {
		return InterceptResult{ShouldSend: false}
	}

	// 2) DEDUP — extrair msg_id do data.Info.ID
	if DedupEnabled() && eventType == "Message" {
		if msgID := extractMsgID(postMap); msgID != "" && IsDuplicateMsgID(msgID) {
			return InterceptResult{ShouldSend: false}
		}
	}

	// 3) Envelope override
	if EnvelopeFormat() == "rproducer-v1" {
		postMap = transformToRproducerV1(postMap)
		newJSON, err := json.Marshal(postMap)
		if err == nil {
			jsonData = newJSON
		}
	}

	// 4) Queue override
	if override := PublishToQueueOverride(); override != "" {
		queueName = override
	}

	return InterceptResult{ShouldSend: true, QueueName: queueName, JSONData: jsonData}
}

// extractMsgID — pega data.Info.ID do postMap (compat com whatsmeow event shape).
func extractMsgID(postMap map[string]interface{}) string {
	data, ok := postMap["data"].(map[string]interface{})
	if !ok {
		return ""
	}
	info, ok := data["Info"].(map[string]interface{})
	if !ok {
		return ""
	}
	id, _ := info["ID"].(string)
	return id
}

// transformToRproducerV1 — reformata o postMap pro envelope que o RProducerAgent
// espera (mesmo formato que o bridge Node.js publicava):
//
//	IN:  { event, instanceToken, instanceId, instanceName, data }
//	OUT: { event, instance, data, timestamp, source }
//
// Field mapping:
//   - event       (lowercase porque o bridge usa minúsculo)
//   - instance    = instanceName (era o nome resolvido via Supabase no bridge)
//   - data        = data (passa direto, já está enriquecido pelo whatsmeow nativo)
//   - timestamp   = ISO-8601 da hora atual
//   - source      = "evolution-go" (compat com o bridge)
func transformToRproducerV1(in map[string]interface{}) map[string]interface{} {
	event, _ := in["event"].(string)
	instanceName, _ := in["instanceName"].(string)
	data := in["data"]

	// O bridge publicava `event` em lowercase ("message", "qrcode", "historysync"...)
	// O whatsmeow nativo usa CamelCase ("Message", "QRCode", "HistorySync").
	// Normalizar pra lowercase pra compat com Agent.
	eventLower := ""
	for _, r := range event {
		if r >= 'A' && r <= 'Z' {
			eventLower += string(r + 32)
		} else {
			eventLower += string(r)
		}
	}

	return map[string]interface{}{
		"event":     eventLower,
		"instance":  instanceName,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"source":    "evolution-go",
	}
}
