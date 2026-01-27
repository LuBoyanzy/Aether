//go:build testing
// +build testing

package agent

import (
	"errors"
	"testing"

	"aether/internal/common"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

// MockHandler for testing
type MockHandler struct {
	requiresVerification bool
	description          string
	handleFunc           func(ctx *HandlerContext) error
}

func (m *MockHandler) Handle(ctx *HandlerContext) error {
	if m.handleFunc != nil {
		return m.handleFunc(ctx)
	}
	return nil
}

func (m *MockHandler) RequiresVerification() bool {
	return m.requiresVerification
}

// TestHandlerRegistry tests the handler registry functionality
func TestHandlerRegistry(t *testing.T) {
	t.Run("default registration", func(t *testing.T) {
		registry := NewHandlerRegistry()

		// Check default handlers are registered
		getDataHandler, exists := registry.GetHandler(common.GetData)
		assert.True(t, exists)
		assert.IsType(t, &GetDataHandler{}, getDataHandler)

		fingerprintHandler, exists := registry.GetHandler(common.CheckFingerprint)
		assert.True(t, exists)
		assert.IsType(t, &CheckFingerprintHandler{}, fingerprintHandler)
	})

	t.Run("custom handler registration", func(t *testing.T) {
		registry := NewHandlerRegistry()
		mockHandler := &MockHandler{
			requiresVerification: true,
			description:          "Test handler",
		}

		// Register a custom handler for a mock action
		const mockAction common.WebSocketAction = 99
		registry.Register(mockAction, mockHandler)

		// Verify registration
		handler, exists := registry.GetHandler(mockAction)
		assert.True(t, exists)
		assert.Equal(t, mockHandler, handler)
	})

	t.Run("unknown action", func(t *testing.T) {
		registry := NewHandlerRegistry()
		ctx := &HandlerContext{
			Request: &common.HubRequest[cbor.RawMessage]{
				Action: common.WebSocketAction(255), // Unknown action
			},
			HubVerified: true,
		}

		err := registry.Handle(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown action: 255")
	})

	t.Run("verification required", func(t *testing.T) {
		registry := NewHandlerRegistry()
		ctx := &HandlerContext{
			Request: &common.HubRequest[cbor.RawMessage]{
				Action: common.GetData, // Requires verification
			},
			HubVerified: false, // Not verified
		}

		err := registry.Handle(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hub not verified")
	})
}

// TestCheckFingerprintHandler tests the CheckFingerprint handler
func TestCheckFingerprintHandler(t *testing.T) {
	handler := &CheckFingerprintHandler{}

	t.Run("handle with invalid data", func(t *testing.T) {
		client := &WebSocketClient{}
		ctx := &HandlerContext{
			Client:      client,
			HubVerified: false,
			Request: &common.HubRequest[cbor.RawMessage]{
				Action: common.CheckFingerprint,
				Data:   cbor.RawMessage{}, // Empty/invalid data
			},
		}

		// Should fail to decode the fingerprint request
		err := handler.Handle(ctx)
		assert.Error(t, err)
	})
}

func TestSendHandlerErrorResponse(t *testing.T) {
	t.Run("send error response", func(t *testing.T) {
		var gotErr error
		var gotID *uint32
		ctx := &HandlerContext{
			SendResponse: func(data any, requestID *uint32) error {
				err, ok := data.(error)
				if ok {
					gotErr = err
				}
				gotID = requestID
				return nil
			},
		}
		reqID := uint32(42)
		originErr := errors.New("operate failed")

		sendErr := sendHandlerErrorResponse(ctx, &reqID, originErr)

		assert.NoError(t, sendErr)
		assert.Equal(t, originErr, gotErr)
		if assert.NotNil(t, gotID) {
			assert.Equal(t, reqID, *gotID)
		}
	})

	t.Run("ignore nil error", func(t *testing.T) {
		ctx := &HandlerContext{
			SendResponse: func(_ any, _ *uint32) error {
				return errors.New("should not be called")
			},
		}
		sendErr := sendHandlerErrorResponse(ctx, nil, nil)
		assert.NoError(t, sendErr)
	})

	t.Run("missing sender", func(t *testing.T) {
		reqID := uint32(7)
		sendErr := sendHandlerErrorResponse(&HandlerContext{}, &reqID, errors.New("boom"))
		assert.Error(t, sendErr)
		assert.Contains(t, sendErr.Error(), "handler response sender not available")
	})
}
