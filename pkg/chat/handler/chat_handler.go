package chat_handler

import (
	"net/http"

	chat_service "github.com/EvolutionAPI/evolution-go/pkg/chat/service"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

type ChatHandler interface {
	ChatPin(ctx *gin.Context)
	ChatUnpin(ctx *gin.Context)
	ChatArchive(ctx *gin.Context)
	ChatUnarchive(ctx *gin.Context)
	ChatMute(ctx *gin.Context)
	ChatUnmute(ctx *gin.Context)
	HistorySyncRequest(ctx *gin.Context)
	// [RPRODUCER] LID → PN resolver (proxy pro LIDStore interno do whatsmeow)
	ResolveLid(ctx *gin.Context)
	// [RPRODUCER] Marca conversa como lida/nao lida via app-state patch (sincroniza com WA Web/Mobile)
	ChatMarkRead(ctx *gin.Context)
	ChatMarkUnread(ctx *gin.Context)
	// [RPRODUCER] Favoritar/labels/presence/receipts/clearChat/block
	ChatStar(ctx *gin.Context)
	ChatLabelEdit(ctx *gin.Context)
	ChatLabelChat(ctx *gin.Context)
	SendPresence(ctx *gin.Context)
	SendChatPresence(ctx *gin.Context)
	ChatMarkMessageRead(ctx *gin.Context)
	ChatClearChat(ctx *gin.Context)
	ChatBlock(ctx *gin.Context)
}

type chatHandler struct {
	chatService chat_service.ChatService
}

// Pin a chat
// @Summary Pin a chat
// @Description Pin a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/pin [post]
func (c *chatHandler) ChatPin(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatPin(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Unpin a chat
// @Summary Unpin a chat
// @Description Unpin a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/unpin [post]
func (c *chatHandler) ChatUnpin(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatUnpin(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Archive a chat
// @Summary Archive a chat
// @Description Archive a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/archive [post]
func (c *chatHandler) ChatArchive(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatArchive(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Unarchive a chat
// @Summary Unarchive a chat
// @Description Unarchive a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/unarchive [post]
func (c *chatHandler) ChatUnarchive(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatUnarchive(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Mute a chat
// @Summary Mute a chat
// @Description Mute a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/mute [post]
func (c *chatHandler) ChatMute(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatMute(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Unmute a chat
// @Summary Unmute a chat
// @Description Unmute a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/unmute [post]
func (c *chatHandler) ChatUnmute(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatUnmute(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// [RPRODUCER] Mark chat as read
// @Summary Mark chat as read
// @Description Marks chat as read via app-state patch (sincroniza com WA Web/Mobile)
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/markRead [post]
func (c *chatHandler) ChatMarkRead(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatMarkRead(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"timestamp": ts}})
}

// [RPRODUCER] Mark chat as unread
// @Summary Mark chat as unread
// @Description Marks chat as unread via app-state patch (sincroniza com WA Web/Mobile). Caso de uso: leadgen.
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/markUnread [post]
func (c *chatHandler) ChatMarkUnread(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatMarkUnread(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"timestamp": ts}})
}

// HistorySyncRequest a chat
// @Summary HistorySyncRequest a chat
// @Description HistorySyncRequest a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.HistorySyncRequestStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/history-sync-request [post]
func (c *chatHandler) HistorySyncRequest(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.HistorySyncRequestStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.chatService.HistorySyncRequest(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// [RPRODUCER] ResolveLid handler
// @Summary Resolve a LID to phone-number JID
// @Description Lookup local no LIDStore do whatsmeow. Retorna o JID real
// @Description (@s.whatsapp.net) correspondente ao LID consultado, ou found=false
// @Description se o whatsmeow ainda não viu esse mapping.
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.ResolveLidStruct true "LID a resolver"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Erro de validação"
// @Failure 500 {object} gin.H "Erro interno"
// @Router /chat/resolveLid [post]
func (c *chatHandler) ResolveLid(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.ResolveLidStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data == nil || data.Lid == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "lid is required"})
		return
	}

	resp, err := c.chatService.ResolveLid(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// [RPRODUCER] helper pra evitar boilerplate nos handlers novos
func bindAndDo[T any](ctx *gin.Context, fn func(*T, *instance_model.Instance) (string, error)) {
	getInstance := ctx.MustGet("instance")
	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}
	var data *T
	if err := ctx.ShouldBindBodyWithJSON(&data); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ts, err := fn(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"timestamp": ts}})
}

// @Summary Star or unstar a message
// @Tags Chat
// @Router /chat/star [post]
func (c *chatHandler) ChatStar(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.ChatStar)
}

// @Summary Create/edit/delete WhatsApp Business label
// @Tags Chat
// @Router /chat/labelEdit [post]
func (c *chatHandler) ChatLabelEdit(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.ChatLabelEdit)
}

// @Summary Apply/remove label to a chat
// @Tags Chat
// @Router /chat/labelChat [post]
func (c *chatHandler) ChatLabelChat(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.ChatLabelChat)
}

// @Summary Send account presence (available/unavailable)
// @Tags Chat
// @Router /chat/presence [post]
func (c *chatHandler) SendPresence(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.SendPresence)
}

// @Summary Send chat presence (typing/recording per chat)
// @Tags Chat
// @Router /chat/chatPresence [post]
func (c *chatHandler) SendChatPresence(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.SendChatPresence)
}

// @Summary Mark messages as read (blue double check)
// @Tags Chat
// @Router /chat/markMessageRead [post]
func (c *chatHandler) ChatMarkMessageRead(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.ChatMarkMessageRead)
}

// @Summary Clear chat history
// @Tags Chat
// @Router /chat/clearChat [post]
func (c *chatHandler) ChatClearChat(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.ChatClearChat)
}

// @Summary Block or unblock a contact
// @Tags Chat
// @Router /chat/block [post]
func (c *chatHandler) ChatBlock(ctx *gin.Context) {
	bindAndDo(ctx, c.chatService.ChatBlock)
}

func NewChatHandler(
	chatService chat_service.ChatService,
) ChatHandler {
	return &chatHandler{
		chatService: chatService,
	}
}
