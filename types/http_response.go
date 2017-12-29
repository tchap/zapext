package types

import (
	"net/http"

	"go.uber.org/zap/zapcore"
)

// HTTPResponse implements zapcore.ObjectMarshaller.
// It is supposed to be used to wrap *http.Response
// so that it can be passed to Zap and marshalled properly:
//
//   zap.Object("http_response", types.HTTPResponse{res})
type HTTPResponse struct {
	R *http.Response
}

// MarshalLogObject implements zapcore.ObjectMarshaller interface.
func (res HTTPResponse) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if res.R == nil {
		return nil
	}

	enc.AddString("status", res.R.Status)
	enc.AddInt("status_code", res.R.StatusCode)
	enc.AddString("proto", res.R.Proto)

	if err := enc.AddReflected("headers", res.R.Header); err != nil {
		return err
	}

	if req := res.R.Request; req != nil {
		if err := enc.AddObject("http_request", HTTPRequest{req}); err != nil {
			return err
		}
	}

	return nil
}
