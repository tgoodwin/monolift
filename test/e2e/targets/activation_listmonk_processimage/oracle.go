package activation_listmonk_processimage

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"os"

	"github.com/disintegration/imaging"
)

const thumbnailSize = 250

type Oracle struct{}

func (Oracle) Invoke(args map[string]any) (any, error) {
	input, err := bytesFromArg(args["input"])
	if err != nil {
		return nil, err
	}
	thumb, width, height, err := processImageBytes(input)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"result0": base64.StdEncoding.EncodeToString(thumb),
		"result1": float64(width),
		"result2": float64(height),
	}, nil
}

func directInvokePayload() map[string]any {
	data, err := os.ReadFile("targets/activation_listmonk_processimage/testdata/fixture.png")
	if err != nil {
		panic(err)
	}
	return map[string]any{"input": data}
}

func processImageBytes(input []byte) ([]byte, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, 0, 0, err
	}
	thumb := imaging.Resize(img, thumbnailSize, 0, imaging.Lanczos)
	var out bytes.Buffer
	if err := imaging.Encode(&out, thumb, imaging.PNG); err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds().Max
	return out.Bytes(), b.X, b.Y, nil
}

func bytesFromArg(v any) ([]byte, error) {
	switch value := v.(type) {
	case []byte:
		return value, nil
	case string:
		return base64.StdEncoding.DecodeString(value)
	default:
		return nil, fmt.Errorf("input must be []byte or base64 string, got %T", v)
	}
}
