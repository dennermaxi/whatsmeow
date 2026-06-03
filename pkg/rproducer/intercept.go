package rproducer

import (
	"context"
	"encoding/json"
	"strings"
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

	// 3) LID Supabase fallback + marker injection
	//    Pra cobrir o caso onde whatsmeow não conseguiu resolver @lid → @s.whatsapp.net
	//    no swap nativo (Sender ou Chat continua @lid). Consulta Clientes.lid_jid.
	if LIDLookupEnabled() && eventType == "Message" {
		enrichLIDInPostMap(postMap)
	}

	// 4) Instance name normalization (slug → Empresa.Nome real)
	if InstanceNameOverride() {
		if rawName, ok := postMap["instanceName"].(string); ok && rawName != "" {
			postMap["instanceName"] = ResolveInstanceName(rawName)
		}
	}

	// 5) Envelope override
	postMapChanged := LIDLookupEnabled() || InstanceNameOverride() // forçou mutação
	if EnvelopeFormat() == "rproducer-v1" {
		postMap = transformToRproducerV1(postMap)
		postMapChanged = true
	}
	if postMapChanged {
		if newJSON, err := json.Marshal(postMap); err == nil {
			jsonData = newJSON
		}
	}

	// 6) Queue override
	if override := PublishToQueueOverride(); override != "" {
		queueName = override
	}

	return InterceptResult{ShouldSend: true, QueueName: queueName, JSONData: jsonData}
}

// enrichLIDInPostMap — inspeciona postMap["data"]["Info"] e tenta resolver
// Sender/Chat/SenderAlt/ChatAlt quando contêm @lid via lookup Supabase.
// Também injeta markers `_enrichedByBridge` e `_realNumber` (compat com Agent
// que pode esperar isso).
func enrichLIDInPostMap(postMap map[string]interface{}) {
	data, ok := postMap["data"].(map[string]interface{})
	if !ok {
		return
	}
	info, ok := data["Info"].(map[string]interface{})
	if !ok {
		return
	}

	// Coleta JIDs @lid presentes
	getStr := func(k string) string { s, _ := info[k].(string); return s }
	candidates := []string{getStr("Sender"), getStr("Chat"), getStr("SenderAlt"), getStr("ChatAlt")}

	var resolvedJid string
	for _, c := range candidates {
		if !strings.Contains(c, "@lid") {
			continue
		}
		if real := ResolveLidViaSupabase(context.Background(), c); real != "" {
			resolvedJid = real
			break
		}
	}
	if resolvedJid == "" {
		return
	}

	// Aplica resolução: troca todos os @lid pelos @s.whatsapp.net real
	// (mantém o @lid no SenderAlt/ChatAlt como referência reversa, igual o bridge fazia)
	realLidJid := "" // o @lid original que resolvemos
	for k, v := range info {
		s, ok := v.(string)
		if !ok {
			continue
		}
		if strings.Contains(s, "@lid") {
			if realLidJid == "" {
				realLidJid = s
			}
			// Se é um campo "principal" (Sender/Chat), troca; SenderAlt/ChatAlt vira o @lid
			switch k {
			case "Sender", "Chat":
				info[k] = resolvedJid
			}
		}
	}
	if realLidJid != "" {
		if _, hasSenderAlt := info["SenderAlt"].(string); !hasSenderAlt || info["SenderAlt"] == "" {
			info["SenderAlt"] = realLidJid
		}
		if _, hasChatAlt := info["ChatAlt"].(string); !hasChatAlt || info["ChatAlt"] == "" {
			info["ChatAlt"] = realLidJid
		}
	}

	// Markers compat com o bridge Node.js
	info["_enrichedByBridge"] = true
	info["_realNumber"] = strings.TrimSuffix(strings.Split(resolvedJid, "@")[0], "")
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
