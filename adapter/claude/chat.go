package claude

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"errors"
	"fmt"
)

const defaultTokens = 2500

func (c *ChatInstance) GetChatEndpoint() string {
	return fmt.Sprintf("%s/v1/messages", c.GetEndpoint())
}

func (c *ChatInstance) GetChatHeaders() map[string]string {
	return map[string]string{
		"content-type":      "application/json",
		"anthropic-version": "2023-06-01",
		"x-api-key":         c.GetApiKey(),
	}
}

// ConvertCompletionMessage converts the completion message to anthropic complete format (deprecated)
func (c *ChatInstance) ConvertCompletionMessage(message []globals.Message) string {
	mapper := map[string]string{
		globals.System:    "Assistant",
		globals.User:      "Human",
		globals.Assistant: "Assistant",
	}

	var result string
	for i, item := range message {
		if item.Role == globals.Tool {
			continue
		}
		if i == 0 && item.Role == globals.Assistant {
			// skip first assistant message
			continue
		}

		result += fmt.Sprintf("\n\n%s: %s", mapper[item.Role], item.Content)
	}
	return fmt.Sprintf("%s\n\nAssistant:", result)
}

func (c *ChatInstance) GetTokens(props *adaptercommon.ChatProps) int {
	if props.MaxTokens == nil || *props.MaxTokens <= 0 {
		return defaultTokens
	}

	return *props.MaxTokens
}

func (c *ChatInstance) GetMessages(props *adaptercommon.ChatProps) []Message {
	return utils.Each(props.Message, func(message globals.Message) Message {
		if !globals.IsVisionModel(props.Model) || message.Role != globals.User {
			return Message{
				Role:    message.Role,
				Content: message.Content,
			}
		}

		content, urls := utils.ExtractImages(message.Content, true)
		images := utils.EachNotNil(urls, func(url string) *MessageContent {
			obj, err := utils.NewImage(url)
			props.Buffer.AddImage(obj)
			if err != nil {
				globals.Info(fmt.Sprintf("cannot process image: %s (source: %s)", err.Error(), utils.Extract(url, 24, "...")))
			}

			i := utils.NewImageContent(url)
			return &MessageContent{
				Type: "image",
				Source: &MessageImage{
					Type:      "base64",
					MediaType: i.GetType(),
					Data:      i.ToRawBase64(),
				},
			}
		})

		return Message{
			Role: message.Role,
			Content: utils.Prepend(images, MessageContent{
				Type: "text",
				Text: &content,
			}),
		}
	})
}

func (c *ChatInstance) GetChatBody(props *adaptercommon.ChatProps, stream bool) *ChatBody {
	messages := c.GetMessages(props)
	return &ChatBody{
		Messages:    messages,
		MaxTokens:   c.GetTokens(props),
		Model:       props.Model,
		Stream:      stream,
		Temperature: props.Temperature,
		TopP:        props.TopP,
		TopK:        props.TopK,
	}
}

func (c *ChatInstance) ProcessLine(data string) (*globals.Chunk, error) {
	if form := processChatResponse(data); form != nil {
		return &globals.Chunk{
			Content: form.Delta.Text,
		}, nil
	}

	if form := processChatErrorResponse(data); form != nil {
		return &globals.Chunk{Content: ""}, fmt.Errorf("anthropic error: %s (type: %s)", form.Error.Message, form.Error.Type)
	}

	return &globals.Chunk{Content: ""}, nil
}

func processChatErrorResponse(data string) *ChatErrorResponse {
	if form := utils.UnmarshalForm[ChatErrorResponse](data); form != nil {
		return form
	}
	return nil
}

func processChatResponse(data string) *ChatStreamResponse {
	if form := utils.UnmarshalForm[ChatStreamResponse](data); form != nil {
		return form
	}
	return nil
}

// CreateStreamChatRequest is the stream request for anthropic claude
func (c *ChatInstance) CreateStreamChatRequest(props *adaptercommon.ChatProps, hook globals.Hook) error {
	err := utils.EventScanner(&utils.EventScannerProps{
		Method:  "POST",
		Uri:     c.GetChatEndpoint(),
		Headers: c.GetChatHeaders(),
		Body:    c.GetChatBody(props, true),
		Callback: func(data string) error {
			partial, err := c.ProcessLine(data)
			if err != nil {
				return err
			}

			return hook(partial)
		},
	},
		props.Proxy,
	)

	if err != nil {
		if form := processChatErrorResponse(err.Body); form != nil {
			if form.Error.Type == "" && form.Error.Message == "" {
				return errors.New(utils.ToMarkdownCode("json", err.Body))
			}

			return errors.New(fmt.Sprintf("%s (type: %s)", form.Error.Message, form.Error.Type))
		}
		return fmt.Errorf("%s\n%s", err.Error, errors.New(utils.ToMarkdownCode("json", err.Body)))
	}

	return nil
}
