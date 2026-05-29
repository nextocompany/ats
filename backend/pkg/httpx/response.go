// Package httpx defines the shared HTTP response envelope and the central error
// handler. Every API response in Sprint 1+ uses this envelope so clients see a
// consistent shape.
package httpx

import "github.com/gofiber/fiber/v2"

// Meta carries pagination metadata for list responses.
type Meta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// Envelope is the consistent wrapper for all responses.
type Envelope[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
}

// OK writes a 200 success envelope.
func OK[T any](c *fiber.Ctx, data T) error {
	return c.Status(fiber.StatusOK).JSON(Envelope[T]{Success: true, Data: data})
}

// Created writes a 201 success envelope.
func Created[T any](c *fiber.Ctx, data T) error {
	return c.Status(fiber.StatusCreated).JSON(Envelope[T]{Success: true, Data: data})
}

// Fail writes an error envelope with the given status and message.
func Fail(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(Envelope[any]{Success: false, Error: msg})
}
