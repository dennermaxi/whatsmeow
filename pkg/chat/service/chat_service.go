package chat_service

import (
	"context"
	"errors"
	"strings"
	"time"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
	whatsmeow_service "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type ChatService interface {
	ChatPin(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatUnpin(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatArchive(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatUnarchive(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatMute(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatUnmute(data *BodyStruct, instance *instance_model.Instance) (string, error)
	HistorySyncRequest(data *HistorySyncRequestStruct, instance *instance_model.Instance) (*whatsmeow.SendResponse, error)
	// [RPRODUCER] ResolveLid — expõe o LIDStore interno do whatsmeow via HTTP.
	// Recebe um JID @lid e retorna o JID real @s.whatsapp.net (ou vazio se desconhecido).
	ResolveLid(data *ResolveLidStruct, instance *instance_model.Instance) (*ResolveLidResponse, error)
	// [RPRODUCER] MarkChatAsRead/MarkChatAsUnread — marca conversa como lida/nao lida.
	// Usa appstate.BuildMarkChatAsRead (whatsmeow) que sincroniza com WA Web/Mobile via patch multi-device.
	// Diferente do Evolution v2 que so atualiza cache interno (nao sincroniza com WA real).
	ChatMarkRead(data *BodyStruct, instance *instance_model.Instance) (string, error)
	ChatMarkUnread(data *BodyStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] Favoritar mensagem (sincroniza com WA — estrela aparece no celular)
	ChatStar(data *StarStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] Labels (etiquetas) — WhatsApp Business. Bidirecional com app.
	ChatLabelEdit(data *LabelEditStruct, instance *instance_model.Instance) (string, error)
	ChatLabelChat(data *LabelChatStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] Presence — online/offline account-wide
	SendPresence(data *PresenceStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] ChatPresence — typing/recording por chat
	SendChatPresence(data *ChatPresenceStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] Mark message read — duplo-check azul (read receipts)
	ChatMarkMessageRead(data *MarkMessageReadStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] Clear chat — limpa historico local (vira BuildDeleteChat na whatsmeow)
	ChatClearChat(data *BodyStruct, instance *instance_model.Instance) (string, error)
	// [RPRODUCER] Block/Unblock contato
	ChatBlock(data *BlockStruct, instance *instance_model.Instance) (string, error)
}

type chatService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	loggerWrapper    *logger_wrapper.LoggerManager
}

type BodyStruct struct {
	Chat string `json:"chat"`
}

type HistorySyncRequestStruct struct {
	MessageInfo *types.MessageInfo `json:"messageInfo"`
	Count       int                `json:"count"`
}

// [RPRODUCER] Request body de POST /chat/resolveLid
type ResolveLidStruct struct {
	Lid string `json:"lid"` // ex: "116419450638400@lid" ou só "116419450638400"
}

// [RPRODUCER] Response body de POST /chat/resolveLid
type ResolveLidResponse struct {
	Lid   string `json:"lid"`   // o LID que foi consultado (normalizado)
	Pn    string `json:"pn"`    // JID real (@s.whatsapp.net) ou string vazia se não achou
	Found bool   `json:"found"` // true se o mapping existe no LIDStore do whatsmeow
}

// [RPRODUCER] POST /chat/star — favoritar/desfavoritar mensagem (sincroniza com WA)
type StarStruct struct {
	Chat      string `json:"chat"`      // jid da conversa
	Sender    string `json:"sender"`    // jid do remetente (mesmo da conversa pra 1-on-1; participante pra grupo)
	MessageId string `json:"messageId"` // id da mensagem
	FromMe    bool   `json:"fromMe"`    // se a msg e da gente ou nao
	Starred   bool   `json:"starred"`   // true=favoritar, false=desfavoritar
}

// [RPRODUCER] POST /chat/labelEdit — cria/edita/deleta label (etiqueta WA Business)
type LabelEditStruct struct {
	LabelId string `json:"labelId"` // id unico da label (string, gerar via uuid ou seq)
	Name    string `json:"name"`    // nome visivel
	Color   int32  `json:"color"`   // 0-19 (paleta WA Business)
	Deleted bool   `json:"deleted"` // true=deletar
}

// [RPRODUCER] POST /chat/labelChat — aplica/remove label numa conversa
type LabelChatStruct struct {
	Chat    string `json:"chat"`    // jid da conversa
	LabelId string `json:"labelId"` // id da label
	Labeled bool   `json:"labeled"` // true=aplicar, false=remover
}

// [RPRODUCER] POST /chat/presence — online/offline account-wide
type PresenceStruct struct {
	Presence string `json:"presence"` // 'available' | 'unavailable'
}

// [RPRODUCER] POST /chat/chatPresence — typing/recording por chat
type ChatPresenceStruct struct {
	Chat  string `json:"chat"`  // jid
	State string `json:"state"` // 'composing' | 'recording' | 'paused'
	Media string `json:"media"` // 'text' | 'audio' (opcional)
}

// [RPRODUCER] POST /chat/markMessageRead — read receipt (duplo-check azul)
type MarkMessageReadStruct struct {
	Chat       string   `json:"chat"`               // jid da conversa
	Sender     string   `json:"sender,omitempty"`   // jid do remetente (default: chat pra 1-on-1)
	MessageIds []string `json:"messageIds"`         // array de IDs
}

// [RPRODUCER] POST /chat/block — bloquear/desbloquear contato
type BlockStruct struct {
	Chat   string `json:"chat"`   // jid do contato
	Action string `json:"action"` // 'block' | 'unblock'
}

// [RPRODUCER v0.7.1-rwa-9] Verifica se o client tem Store.AppState carregado.
// whatsmeow appstate.go:503 (cli.Store.AppState.GetAppStateVersion) panica com
// nil pointer se Store ou AppState forem nil — pode acontecer logo apos
// reconexao quando session ainda nao tem AppState totalmente inicializado.
func clientHasAppState(client *whatsmeow.Client) bool {
	return client != nil && client.Store != nil && client.Store.AppState != nil
}

// [RPRODUCER v0.7.1-rwa-9] ensureClientForAppState — combina ensureClientConnected
// + check de AppState. Retorna erro tratado em vez de panic. Use em funcoes que
// chamam cli.SendAppState().
func (c *chatService) ensureClientForAppState(instanceId string) (*whatsmeow.Client, error) {
	client, err := c.ensureClientConnected(instanceId)
	if err != nil {
		return nil, err
	}
	if !clientHasAppState(client) {
		return nil, errors.New("client app state not initialized — try again in a few seconds")
	}
	return client, nil
}

func (c *chatService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := c.clientPointer[instanceId]
	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := c.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = c.clientPointer[instanceId]
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (c *chatService) ChatPin(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildPin(recipient, true))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error pin chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatUnpin(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildPin(recipient, false))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unpin chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatArchive(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildArchive(recipient, true, time.Time{}, nil))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error archive chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatUnarchive(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildArchive(recipient, false, time.Time{}, nil))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unarchive chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatMute(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildMute(recipient, true, 1*time.Hour))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mute chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

func (c *chatService) ChatUnmute(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildMute(recipient, false, 0*time.Hour))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unmute chat: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

// [RPRODUCER] ChatMarkRead — marca conversa como LIDA (sincroniza com WA Web/mobile via app-state patch).
func (c *chatService) ChatMarkRead(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildMarkChatAsRead(recipient, true, time.Time{}, nil))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mark chat as read: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

// [RPRODUCER] ChatMarkUnread — marca conversa como NAO LIDA (sincroniza com WA Web/mobile via app-state patch).
// Caso de uso: leadgen — apos sistema responder ao lead, marcar conversa como nao lida pra atendente humano notar.
func (c *chatService) ChatMarkUnread(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}

	err = client.SendAppState(context.Background(), appstate.BuildMarkChatAsRead(recipient, false, time.Time{}, nil))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mark chat as unread: %v", instance.Id, err)
		return "", err
	}

	return ts.String(), nil
}

// [RPRODUCER] ChatStar — favoritar/desfavoritar msg (sincroniza estrela com WA)
func (c *chatService) ChatStar(data *StarStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	chat, ok := utils.ParseJID(data.Chat)
	if !ok {
		return "", errors.New("invalid chat jid")
	}
	sender := chat
	if data.Sender != "" {
		s, ok2 := utils.ParseJID(data.Sender)
		if ok2 {
			sender = s
		}
	}
	if data.MessageId == "" {
		return "", errors.New("messageId is required")
	}
	err = client.SendAppState(context.Background(), appstate.BuildStar(chat, sender, data.MessageId, data.FromMe, data.Starred))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error star msg: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] ChatLabelEdit — cria/edita/deleta label (etiqueta WA Business)
func (c *chatService) ChatLabelEdit(data *LabelEditStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	if data.LabelId == "" {
		return "", errors.New("labelId is required")
	}
	err = client.SendAppState(context.Background(), appstate.BuildLabelEdit(data.LabelId, data.Name, data.Color, data.Deleted))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error label edit: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] ChatLabelChat — aplica/remove label numa conversa
func (c *chatService) ChatLabelChat(data *LabelChatStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	chat, ok := utils.ParseJID(data.Chat)
	if !ok {
		return "", errors.New("invalid chat jid")
	}
	if data.LabelId == "" {
		return "", errors.New("labelId is required")
	}
	err = client.SendAppState(context.Background(), appstate.BuildLabelChat(chat, data.LabelId, data.Labeled))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error label chat: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] SendPresence — account-wide online/offline
func (c *chatService) SendPresence(data *PresenceStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	var presence types.Presence
	switch strings.ToLower(data.Presence) {
	case "available", "online":
		presence = types.PresenceAvailable
	case "unavailable", "offline":
		presence = types.PresenceUnavailable
	default:
		return "", errors.New("presence must be 'available' or 'unavailable'")
	}
	err = client.SendPresence(context.Background(), presence)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error send presence: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] SendChatPresence — typing/recording em chat especifico
func (c *chatService) SendChatPresence(data *ChatPresenceStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	chat, ok := utils.ParseJID(data.Chat)
	if !ok {
		return "", errors.New("invalid chat jid")
	}
	var state types.ChatPresence
	switch strings.ToLower(data.State) {
	case "composing", "typing":
		state = types.ChatPresenceComposing
	case "paused", "stop":
		state = types.ChatPresencePaused
	case "recording":
		state = types.ChatPresenceComposing
	default:
		return "", errors.New("state must be 'composing', 'paused' or 'recording'")
	}
	var media types.ChatPresenceMedia
	switch strings.ToLower(data.Media) {
	case "audio", "recording":
		media = types.ChatPresenceMediaAudio
	default:
		media = types.ChatPresenceMediaText
	}
	err = client.SendChatPresence(context.Background(), chat, state, media)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error send chat presence: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] ChatMarkMessageRead — read receipts (duplo-check azul)
func (c *chatService) ChatMarkMessageRead(data *MarkMessageReadStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	chat, ok := utils.ParseJID(data.Chat)
	if !ok {
		return "", errors.New("invalid chat jid")
	}
	sender := chat
	if data.Sender != "" {
		s, ok2 := utils.ParseJID(data.Sender)
		if ok2 {
			sender = s
		}
	}
	if len(data.MessageIds) == 0 {
		return "", errors.New("messageIds is required")
	}
	ids := make([]types.MessageID, 0, len(data.MessageIds))
	for _, id := range data.MessageIds {
		if id != "" {
			ids = append(ids, types.MessageID(id))
		}
	}
	err = client.MarkRead(context.Background(), ids, time.Now(), chat, sender)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mark read: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] ChatClearChat — limpa historico (BuildDeleteChat na whatsmeow)
func (c *chatService) ChatClearChat(data *BodyStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientForAppState(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	chat, ok := utils.ParseJID(data.Chat)
	if !ok {
		return "", errors.New("invalid chat jid")
	}
	err = client.SendAppState(context.Background(), appstate.BuildDeleteChat(chat, time.Time{}, nil, false))
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error clear chat: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

// [RPRODUCER] ChatBlock — bloqueia/desbloqueia contato
func (c *chatService) ChatBlock(data *BlockStruct, instance *instance_model.Instance) (string, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}
	var ts time.Time
	chat, ok := utils.ParseJID(data.Chat)
	if !ok {
		return "", errors.New("invalid chat jid")
	}
	var action events.BlocklistChangeAction
	switch strings.ToLower(data.Action) {
	case "block":
		action = events.BlocklistChangeActionBlock
	case "unblock":
		action = events.BlocklistChangeActionUnblock
	default:
		return "", errors.New("action must be 'block' or 'unblock'")
	}
	_, err = client.UpdateBlocklist(context.Background(), chat, action)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error update blocklist: %v", instance.Id, err)
		return "", err
	}
	return ts.String(), nil
}

func (c *chatService) HistorySyncRequest(data *HistorySyncRequestStruct, instance *instance_model.Instance) (*whatsmeow.SendResponse, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	messageInfo := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     data.MessageInfo.Chat,
			IsFromMe: data.MessageInfo.IsFromMe,
			IsGroup:  data.MessageInfo.IsGroup,
		},
		ID:        data.MessageInfo.ID,
		Timestamp: data.MessageInfo.Timestamp,
	}

	histRequest := client.BuildHistorySyncRequest(&messageInfo, data.Count)

	res, err := client.SendMessage(context.Background(), messageInfo.Chat, histRequest, whatsmeow.SendRequestExtra{Peer: true})
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error history sync request: %v", instance.Id, err)
		return nil, err
	}

	return &res, nil
}

// [RPRODUCER] ResolveLid — chama client.Store.LIDs.GetPNForLID()
// ----------------------------------------------------------------
// Pattern: o whatsmeow mantém internamente uma LIDStore (em SQLite/Postgres
// dependendo do storage configurado). A store é populada sempre que o
// whatsmeow recebe mensagens onde tanto SenderAlt quanto Sender estão
// presentes (ou via outros caminhos como GroupParticipant).
//
// Esse endpoint é um lookup local (sem ida pra rede), idempotente, super rápido.
// O cliente CRM usa pra resolver LIDs órfãos que vieram de sync histórico
// quando a store ainda não tinha o mapping.
func (c *chatService) ResolveLid(data *ResolveLidStruct, instance *instance_model.Instance) (*ResolveLidResponse, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	// Aceita "X@lid" ou só "X" — normaliza pra JID
	lidStr := data.Lid
	if !strings.Contains(lidStr, "@") {
		lidStr = lidStr + "@lid"
	}
	lidJID, err := types.ParseJID(lidStr)
	if err != nil {
		return nil, errors.New("lid inválido: " + err.Error())
	}
	if lidJID.Server != "lid" {
		return nil, errors.New("JID precisa ser @lid (recebido: " + lidJID.Server + ")")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pnJID, err := client.Store.LIDs.GetPNForLID(ctx, lidJID)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] GetPNForLID erro lid=%s: %v", instance.Id, lidStr, err)
		return nil, err
	}

	resp := &ResolveLidResponse{Lid: lidStr}
	if pnJID.IsEmpty() {
		resp.Found = false
		resp.Pn = ""
	} else {
		resp.Found = true
		resp.Pn = pnJID.String()
	}
	return resp, nil
}

func NewChatService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
) ChatService {
	return &chatService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
	}
}
