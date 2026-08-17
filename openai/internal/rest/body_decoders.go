package rest

import "github.com/getkin/kin-openapi/openapi3filter"

func init() {
	// The frozen OpenAI contract uses raw SDP request bodies and typed binary
	// multipart parts. kin-openapi v0.135 only registers generic text and
	// octet-stream decoders, so register the equivalent decoders for the media
	// types emitted by the pinned official SDK.
	openapi3filter.RegisterBodyDecoder("application/sdp", openapi3filter.PlainBodyDecoder)
	for _, mediaType := range []string{
		"audio/mpeg",
		"audio/mp4",
		"audio/wav",
		"image/jpeg",
		"image/png",
		"video/mp4",
	} {
		openapi3filter.RegisterBodyDecoder(mediaType, openapi3filter.FileBodyDecoder)
	}
}
