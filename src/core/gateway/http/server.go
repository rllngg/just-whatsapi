package http

import (
	"context"

	"whatsapp/src/core/whatsapp"

	"github.com/gofiber/fiber/v2"
)

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
	s.app.Post("/device/new", s.handleNewDevice)
	s.app.Get("/device", s.handleGetDevices)

	// Message endpoints
	s.app.Post("/message/text", s.handleSendTextMessage)
	s.app.Post("/message/image", s.handleSendImageMessage)
	s.app.Post("/message/file", s.handleSendFileMessage)
	s.app.Post("/message/presence", s.handleSendPresence)
	s.app.Post("/message/emoji", s.handleSendReaction)
}

// handleNewDevice creates a new WhatsApp device and returns QR code
func (s *Server) handleNewDevice(c *fiber.Ctx) error {
	ctx := context.Background()
	device, err := s.manager.CreateDevice(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"qr_code": device.QRCode,
	})
}

// handleGetDevices returns all devices
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
