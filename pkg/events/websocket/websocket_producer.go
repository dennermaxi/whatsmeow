package websocket_producer

import (
	"net/http"
	"strings"
	"sync"

	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	"github.com/gomessguii/logger"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		logger.LogInfo("Verificando origem da conexão WebSocket")
		return true
	},
}

// [RPRODUCER] safeConn — wrapper thread-safe pra *websocket.Conn
// ----------------------------------------------------------------
// A gorilla/websocket não é thread-safe pra writes concorrentes (panic ou
// conexão corrompida quando 2+ goroutines chamam WriteJSON ao mesmo tempo).
// Como o producer.Produce() pode ser invocado por handlers diferentes (eventos
// vindos do whatsmeow + n8n consumers + healthcheck) pra mesma instância,
// precisamos serializar os writes via Mutex.
//
// Por que NÃO mover o Mutex pro websocketProducer.clientsMux? Porque o RLock
// não serializa writes em conns individuais; só protege o map de clientes.
// Cada client.Conn precisa do próprio mutex.
//
// Pattern padrão recomendado pela própria doc do gorilla:
//   https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency
type safeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// [RPRODUCER] newSafeConn — constructor pro wrapper
func newSafeConn(conn *websocket.Conn) *safeConn {
	return &safeConn{conn: conn}
}

// [RPRODUCER] WriteJSON — serializado com mutex (defer Unlock garante release
// mesmo se a serialização entrar em panic)
func (sc *safeConn) WriteJSON(v interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteJSON(v)
}

// [RPRODUCER] ReadMessage — sem mutex porque gorilla aceita 1 leitor + 1
// escritor concorrentes (só múltiplos escritores que quebram)
func (sc *safeConn) ReadMessage() (messageType int, p []byte, err error) {
	return sc.conn.ReadMessage()
}

// [RPRODUCER] Close — serializado pra não dar race com WriteJSON em voo
func (sc *safeConn) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.Close()
}

type websocketProducer struct {
	// [RPRODUCER] trocado *websocket.Conn → *safeConn em ambos os mapas
	clients       map[string]*safeConn // conexões específicas por instância
	broadcast     []*safeConn          // conexões que recebem todos os eventos
	clientsMux    sync.RWMutex
	loggerWrapper *logger_wrapper.LoggerManager
}

func NewWebsocketProducer(loggerWrapper *logger_wrapper.LoggerManager) *websocketProducer {
	return &websocketProducer{
		clients:       make(map[string]*safeConn),
		broadcast:     make([]*safeConn, 0),
		clientsMux:    sync.RWMutex{},
		loggerWrapper: loggerWrapper,
	}
}

// ServeWs lida com as requisições de upgrade para websocket
func ServeWs(w http.ResponseWriter, r *http.Request, instanceId string, producer *websocketProducer) {
	logger.LogInfo("Iniciando upgrade da conexão WebSocket")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.LogError("Erro ao fazer upgrade da conexão websocket: %v", err)
		return
	}

	logger.LogInfo("Conexão WebSocket estabelecida com sucesso")

	// [RPRODUCER] wrappar a conn na hora do upgrade
	sc := newSafeConn(conn)

	if instanceId == "" {
		producer.AddBroadcastClient(sc)
	} else {
		producer.AddClient(instanceId, sc)
	}

	// Goroutine para limpar conexão quando fechada
	go func() {
		for {
			_, _, err := sc.ReadMessage()
			if err != nil {
				if instanceId == "" {
					producer.RemoveBroadcastClient(sc)
				} else {
					producer.RemoveClient(instanceId)
				}
				sc.Close()
				break
			}
		}
	}()
}

func (p *websocketProducer) AddBroadcastClient(conn *safeConn) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	p.broadcast = append(p.broadcast, conn)
	logger.LogInfo("Cliente broadcast websocket adicionado")
}

func (p *websocketProducer) RemoveBroadcastClient(conn *safeConn) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	for i, c := range p.broadcast {
		if c == conn {
			p.broadcast = append(p.broadcast[:i], p.broadcast[i+1:]...)
			break
		}
	}
	logger.LogInfo("Cliente broadcast websocket removido")
}

func (p *websocketProducer) AddClient(instanceID string, conn *safeConn) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	p.clients[instanceID] = conn
	p.loggerWrapper.GetLogger(instanceID).LogInfo("Cliente websocket adicionado para instância: %s", instanceID)
}

func (p *websocketProducer) RemoveClient(instanceID string) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	delete(p.clients, instanceID)
	p.loggerWrapper.GetLogger(instanceID).LogInfo("Cliente websocket removido para instância: %s", instanceID)
}

func (p *websocketProducer) Produce(queueName string, payload []byte, instanceID string, _ string) error {
	message := map[string]interface{}{
		"queue":   strings.ToLower(queueName),
		"payload": string(payload),
	}

	p.clientsMux.RLock()
	defer p.clientsMux.RUnlock()

	// Envia para cliente específico da instância
	if client, exists := p.clients[instanceID]; exists {
		// [RPRODUCER] WriteJSON do safeConn — serializado por mutex
		err := client.WriteJSON(message)
		if err != nil {
			p.loggerWrapper.GetLogger(instanceID).LogError("Erro ao enviar mensagem websocket para %s: %v", instanceID, err)
			// Não remove o cliente aqui pois estamos com o RLock
			return err
		}
		p.loggerWrapper.GetLogger(instanceID).LogInfo("Mensagem websocket enviada com sucesso para instância %s na fila %s", instanceID, queueName)
	}

	// Envia para todos os clientes broadcast
	for _, conn := range p.broadcast {
		err := conn.WriteJSON(message)
		if err != nil {
			p.loggerWrapper.GetLogger(instanceID).LogError("Erro ao enviar mensagem broadcast websocket: %v", err)
			continue
		}
	}

	return nil
}

// CreateGlobalQueues não faz nada para websocket producer
func (p *websocketProducer) CreateGlobalQueues() error {
	return nil
}
