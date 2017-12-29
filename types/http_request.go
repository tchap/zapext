package types

import (
	"net/http"

	"go.uber.org/zap/zapcore"
)

// HTTPRequest implements zapcore.ObjectMarshaller.
// It is supposed to be used to wrap *http.Request
// so that it can be passed to Zap and marshalled properly:
//
//   zap.Object("http_request", types.HTTPRequest{req})
type HTTPRequest struct {
	R *http.Request
}

// MarshalLogObject implements zapcore.ObjectMarshaller interface.
func (req HTTPRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if req.R == nil {
		return nil
	}

	enc.AddString("method", req.R.Method)
	enc.AddString("url", req.R.URL.String())
	enc.AddString("proto", req.R.Proto)

	if err := enc.AddReflected("headers", req.R.Header); err != nil {
		return err
	}

	enc.AddString("host", req.R.Host)

	if req.R.Form != nil {
		if err := enc.AddReflected("form", req.R.Form); err != nil {
			return err
		}
	}

	if req.R.PostForm != nil {
		if err := enc.AddReflected("post_form", req.R.PostForm); err != nil {
			return err
		}
	}

	if addr := req.R.RemoteAddr; addr != "" {
		enc.AddString("remote_addr", addr)
	}

	if uri := req.R.RequestURI; uri != "" {
		enc.AddString("request_uri", uri)
	}

	return nil
}
