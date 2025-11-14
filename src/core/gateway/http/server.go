package http

import (
	"context"

	"whatsapp/src/core/whatsapp"

	_ "whatsapp/docs" // Import generated Swagger docs

	"github.com/gofiber/fiber/v2"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

// @title WhatsApp Gateway API
// @version 1.0
// @description API for managing WhatsApp devices and sending messages
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
// @schemes http https

// Server handles HTTP requests using Fiber for WhatsApp device management
type Server struct {
	manager *whatsapp.Manager
	app     *fiber.App
}

// NewServer creates a new Fiber HTTP server
func NewServer(manager *whatsapp.Manager) *Server {
	s := &Server{
		manager: manager,
		app: fiber.New(fiber.Config{
			DisableStartupMessage: true,
		}),
	}

	s.registerRoutes()
	return s
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Swagger documentation
	s.app.Get("/swagger/*", fiberSwagger.FiberWrapHandler(fiberSwagger.URL("/swagger/doc.json")))

	// Device endpoints
	s.app.Post("/device/new", s.handleNewDevice)
	s.app.Get("/device", s.handleGetDevices)

	// Message endpoints
	s.app.Post("/message/text", s.handleSendTextMessage)
	s.app.Post("/message/image", s.handleSendImageMessage)
	s.app.Post("/message/file", s.handleSendFileMessage)
	s.app.Post("/message/presence", s.handleSendPresence)
	s.app.Post("/message/emoji", s.handleSendReaction)
	s.app.Post("/message/history", s.handleFetchMessageHistory)
}

// CreateDeviceRequest represents a request to create a new device
type CreateDeviceRequest struct {
	DeviceID string `json:"device_id,omitempty"` // Optional device ID
}

// handleNewDevice creates a new WhatsApp device and returns QR code
// @Summary Create new WhatsApp device
// @Description Creates a new WhatsApp device and returns a QR code for authentication. You can optionally provide a device_id, otherwise one will be auto-generated.
// @Tags Device
// @Accept json
// @Produce json
// @Param request body CreateDeviceRequest false "Device creation request"
// @Success 200 {object} map[string]string "Returns device_id and base64 encoded QR code"
// @Failure 400 {object} map[string]string "Invalid request or device ID already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /device/new [post]
func (s *Server) handleNewDevice(c *fiber.Ctx) error {
	var req CreateDeviceRequest
	// Parse request body if present (device_id is optional)
	if err := c.BodyParser(&req); err != nil && err != fiber.ErrUnprocessableEntity {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	device, err := s.manager.CreateDevice(ctx, req.DeviceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"device_id": device.ID,
		"qr_code":   device.QRCode,
	})
}

// handleGetDevices returns all devices
// @Summary Get all devices
// @Description Returns a list of all registered WhatsApp devices
// @Tags Device
// @Accept json
// @Produce json
// @Success 200 {array} map[string]interface{} "List of devices with id and status"
// @Router /device [get]
func (s *Server) handleGetDevices(c *fiber.Ctx) error {
	devices := s.manager.GetDevices()

	response := make([]fiber.Map, 0, len(devices))
	for _, device := range devices {
		response = append(response, fiber.Map{
			"id":     device.ID,
			"status": device.Status,
		})
	}

	return c.JSON(response)
}

// Listen starts the Fiber server on the given address
func (s *Server) Listen(addr string) error {
	return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

// App returns the Fiber app instance (useful for testing)
func (s *Server) App() *fiber.App {
	return s.app
}

// handleSendTextMessage handles text message sending
// @Summary Send text message
// @Description Sends a text message to a WhatsApp chat
// @Tags Message
// @Accept json
// @Produce json
// @Param request body whatsapp.SendTextMessageRequest true "Text message request"
// @Success 200 {object} whatsapp.MessageResponse "Message sent successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /message/text [post]
func (s *Server) handleSendTextMessage(c *fiber.Ctx) error {
	var req whatsapp.SendTextMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	resp, err := s.manager.SendTextMessage(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}

// handleSendImageMessage handles image message sending
// @Summary Send image message
// @Description Sends an image message to a WhatsApp chat
// @Tags Message
// @Accept json
// @Produce json
// @Param request body whatsapp.SendImageMessageRequest true "Image message request"
// @Success 200 {object} whatsapp.MessageResponse "Image sent successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /message/image [post]
func (s *Server) handleSendImageMessage(c *fiber.Ctx) error {
	var req whatsapp.SendImageMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	resp, err := s.manager.SendImageMessage(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}

// handleSendFileMessage handles file message sending
// @Summary Send file message
// @Description Sends a file/document message to a WhatsApp chat
// @Tags Message
// @Accept json
// @Produce json
// @Param request body whatsapp.SendFileMessageRequest true "File message request"
// @Success 200 {object} whatsapp.MessageResponse "File sent successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /message/file [post]
func (s *Server) handleSendFileMessage(c *fiber.Ctx) error {
	var req whatsapp.SendFileMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	resp, err := s.manager.SendFileMessage(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}

// handleSendPresence handles presence update
// @Summary Send presence update
// @Description Sends a typing indicator/presence update to a WhatsApp chat
// @Tags Message
// @Accept json
// @Produce json
// @Param request body whatsapp.SendPresenceRequest true "Presence request"
// @Success 200 {object} map[string]string "Presence sent successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /message/presence [post]
func (s *Server) handleSendPresence(c *fiber.Ctx) error {
	var req whatsapp.SendPresenceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	err := s.manager.SendPresence(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message_id": "OK",
	})
}

// handleSendReaction handles reaction sending
// @Summary Send emoji reaction
// @Description Sends an emoji reaction to a specific message in a WhatsApp chat
// @Tags Message
// @Accept json
// @Produce json
// @Param request body whatsapp.SendReactionRequest true "Reaction request"
// @Success 200 {object} whatsapp.MessageResponse "Reaction sent successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /message/emoji [post]
func (s *Server) handleSendReaction(c *fiber.Ctx) error {
	var req whatsapp.SendReactionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	resp, err := s.manager.SendReaction(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}

// handleFetchMessageHistory fetches message history from WhatsApp
// @Summary Fetch message history
// @Description Requests message history from the user's primary WhatsApp device. The response indicates the request was sent. Actual messages will arrive via events.HistorySync events and webhooks. Supports pagination using before_message_id.
// @Tags Message
// @Accept json
// @Produce json
// @Param request body whatsapp.FetchMessageHistoryRequest true "Message history request with device_id, chat_id, optional before_message_id for pagination, and count (default: 50, max: 100)"
// @Success 200 {object} whatsapp.MessageHistoryResponse "History sync request sent successfully. Contains request_id for tracking."
// @Failure 400 {object} map[string]string "Invalid request body or parameters"
// @Failure 500 {object} map[string]string "Internal server error or device not connected"
// @Router /message/history [post]
func (s *Server) handleFetchMessageHistory(c *fiber.Ctx) error {
	var req whatsapp.FetchMessageHistoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx := context.Background()
	resp, err := s.manager.FetchMessageHistory(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}
