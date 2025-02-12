package vision

import (
	"context"

	ocr "github.com/yandex-cloud/go-genproto/yandex/cloud/ai/ocr/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
)

func DetectLicensePlates(ctx context.Context, mimeType string, content []byte) ([]string, error) {
	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		Credentials: ycsdk.InstanceServiceAccount(),
	})
	if err != nil {
		return nil, err
	}
	request := &ocr.RecognizeTextRequest{}
	request.SetLanguageCodes([]string{"en", "ru"})
	request.SetModel("license-plates")
	request.SetMimeType(mimeType)
	request.SetContent(content)
	responder, err := sdk.AI().OCR().TextRecognition().Recognize(ctx, request)
	if err != nil {
		return nil, err
	}
	defer responder.CloseSend()
	response, err := responder.Recv()
	if err != nil {
		return nil, err
	}
	var output []string
	for _, block := range response.GetTextAnnotation().GetBlocks() {
		for _, line := range block.GetLines() {
			output = append(output, line.Text)
		}
	}
	return output, nil
}
