package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const MaxLocalImageBytes int64 = 20 << 20

const VideoTaskPathPrefix = "/api/v1/tasks/"

const (
	ChatProtocol   = "openai.chat.v1"
	ImageProtocol  = "dashscope.image.v1"
	VideoProtocol  = "dashscope.media-video.v1"
	SpeechProtocol = "dashscope.speech.v1"
)

// OperationBinding is a versioned, explicit wire contract and its model constraints.
// RemoteID never participates in capability resolution.
type OperationBinding struct {
	ImageModes       []string           `json:"image_modes,omitempty"`
	MinReferences    int                `json:"min_references,omitempty"`
	Contract         *OperationContract `json:"contract,omitempty"`
	Custom           *CustomInterface   `json:"custom,omitempty"`
	Protocol         string             `json:"protocol"`
	Source           string             `json:"source"`
	EndpointMode     string             `json:"endpoint_mode,omitempty"`
	Endpoint         string             `json:"endpoint,omitempty"`
	InputModes       []string           `json:"input_modes,omitempty"`
	InputMode        string             `json:"input_mode,omitempty"`
	Ratios           []string           `json:"ratios,omitempty"`
	Durations        []int              `json:"durations,omitempty"`
	Resolutions      []string           `json:"resolutions,omitempty"`
	ImageSizes       map[string]string  `json:"image_sizes,omitempty"`
	MaxReferences    int                `json:"max_references,omitempty"`
	MaxImageBytes    int64              `json:"max_image_bytes,omitempty"`
	Voices           []Voice            `json:"voices,omitempty"`
	MaxTextLength    int                `json:"max_text_length,omitempty"`
	LastFrame        bool               `json:"last_frame,omitempty"`
	DrivingAudio     bool               `json:"driving_audio,omitempty"`
	MaxAudioBytes    int64              `json:"max_audio_bytes,omitempty"`
	MinAudioDuration int                `json:"min_audio_duration,omitempty"`
	MaxAudioDuration int                `json:"max_audio_duration,omitempty"`
}

type OperationStatus struct {
	Available bool             `json:"available"`
	Code      string           `json:"code"`
	Reason    string           `json:"reason"`
	Binding   OperationBinding `json:"binding"`
}

type ProtocolDescriptor struct {
	Version           int             `json:"version,omitempty"`
	ExecutionModes    []ExecutionMode `json:"execution_modes,omitempty"`
	TemplateVariables []string        `json:"template_variables,omitempty"`
	ID                string          `json:"id"`
	Operation         Capability      `json:"operation"`
	Name              string          `json:"name"`
	DefaultPath       string          `json:"default_path"`
	PollingPath       string          `json:"polling_path,omitempty"`
}

// Descriptors describe implemented formats, not provider model families.
func Protocols() []ProtocolDescriptor {
	items := []ProtocolDescriptor{
		{ID: ChatProtocol, Operation: Story, Name: "OpenAI Chat Completions", DefaultPath: "/compatible-mode/v1/chat/completions"},
		{ID: ImageProtocol, Operation: Image, Name: "DashScope 多模态图片", DefaultPath: "/api/v1/services/aigc/multimodal-generation/generation"},
		{ID: VideoProtocol, Operation: Video, Name: "DashScope media 视频（异步）", DefaultPath: "/api/v1/services/aigc/video-generation/video-synthesis", PollingPath: VideoTaskPathPrefix + "{task_id}"},
		{ID: SpeechProtocol, Operation: Speech, Name: "DashScope 语音合成", DefaultPath: "/api/v1/services/aigc/multimodal-generation/generation"},
	}
	for _, op := range []Capability{Story, Image, Video, Speech} {
		modes := []ExecutionMode{SyncJSON, SSEStream, NDJSONStream}
		if op == Video {
			modes = append(modes, AsyncPolling)
		}
		items = append(items, ProtocolDescriptor{ID: CustomProtocol, Version: 1, Operation: op, Name: "自定义接口（JSON）", TemplateVariables: CustomVariables(op), ExecutionModes: modes})
	}
	return items
}

// RequestPath is shared by execution and the protocol catalogue shown in the UI.
// An absent mode preserves legacy endpoint overrides; explicit default ignores them.
func (b OperationBinding) RequestPath() string {
	if b.Protocol == CustomProtocol && b.Custom != nil {
		return b.Custom.Path
	}
	if b.EndpointMode == "custom" || (b.EndpointMode == "" && b.Endpoint != "") {
		return b.Endpoint
	}
	for _, protocol := range Protocols() {
		if protocol.ID == b.Protocol {
			return protocol.DefaultPath
		}
	}
	return ""
}

func (b OperationBinding) Validate(operation Capability) error {
	found := false
	for _, p := range Protocols() {
		if p.ID == b.Protocol && p.Operation == operation {
			found = true
		}
	}
	if !found {
		return errors.New("protocol_unsupported: 请为该能力选择已实现的接口协议")
	}
	if b.Protocol == CustomProtocol {
		if b.Custom == nil {
			return errors.New("请配置自定义接口格式")
		}
		if b.Endpoint != "" || b.EndpointMode == "custom" {
			return errors.New("自定义接口请使用独立的请求配置，不使用旧路径覆盖")
		}
		if err := b.Custom.Validate(operation); err != nil {
			return err
		}
		variables, _ := b.Custom.Variables()
		if operation == Image && b.SupportsImageMode(ImageToImage) && !variables["images"] {
			return errors.New("参考图能力需要在请求模板映射 {{images}}")
		}
		if operation == Video {
			for _, mode := range b.VideoInputModes() {
				required := "first_frame"
				if mode == "reference_images" {
					required = "images"
				}
				if !variables[required] {
					return fmt.Errorf("视频模板需要映射 {{%s}}", required)
				}
			}
			if b.LastFrame && !variables["last_frame"] {
				return errors.New("尾帧能力需要映射 {{last_frame}}")
			}
			if b.DrivingAudio {
				return errors.New("自定义接口暂不支持驱动音频上传")
			}
		}
	} else if b.Custom != nil {
		return errors.New("内置协议不能附带自定义接口格式")
	}
	if b.EndpointMode != "" && b.EndpointMode != "default" && b.EndpointMode != "custom" {
		return errors.New("请选择协议默认路径或自定义路径")
	}
	if b.EndpointMode == "custom" && strings.TrimSpace(b.Endpoint) == "" {
		return errors.New("自定义路径不能为空")
	}
	if b.EndpointMode != "default" && b.Endpoint != "" && (!strings.HasPrefix(b.Endpoint, "/") || strings.HasPrefix(b.Endpoint, "//") || strings.ContainsAny(b.Endpoint, "?#\\") || strings.Contains(b.Endpoint, "..")) {
		return errors.New("接口路径必须为不含查询参数的绝对路径")
	}
	if operation == Image || operation == Video {
		if err := b.validateRatios(); err != nil {
			return err
		}
		if b.MaxReferences < 0 || b.MaxReferences > 100 {
			return errors.New("参考图片数量须在 0 至 100 之间")
		}
	}
	switch operation {
	case Image:
		if err := b.validateImageModes(); err != nil {
			return err
		}
		for _, r := range b.Ratios {
			var w, h int
			if _, e := fmt.Sscanf(b.ImageSizes[r], "%d*%d", &w, &h); e != nil || w <= 0 || h <= 0 || w > 16384 || h > 16384 || b.ImageSizes[r] != fmt.Sprintf("%d*%d", w, h) {
				return errors.New("每个比例需配置有效图片尺寸，例如 1664*928")
			}
		}
	case Video:
		modes := b.VideoInputModes()
		if len(modes) == 0 {
			return errors.New("请至少选择一种视频输入方式")
		}
		for i, mode := range modes {
			if mode != "first_frame" && mode != "reference_images" {
				return errors.New("不支持的视频输入方式")
			}
			if slices.Contains(modes[:i], mode) {
				return errors.New("视频输入方式不能重复")
			}
		}
		if b.SupportsVideoMode("reference_images") && b.MaxReferences < 1 {
			return errors.New("多图参考需至少一张图片")
		}
		if !b.SupportsVideoMode("first_frame") && (b.LastFrame || b.DrivingAudio) {
			return errors.New("尾帧与驱动音频需要启用首尾帧模式")
		}
		if b.MaxImageBytes > MaxLocalImageBytes {
			return errors.New("当前本地图片传输上限为 20 MB，请收窄配置")
		}
		if b.MaxImageBytes <= 0 || len(b.Durations) == 0 || len(b.Resolutions) == 0 {
			return errors.New("请配置时长、清晰度和图片大小上限")
		}
		for _, d := range b.Durations {
			if d <= 0 || d > 3600 {
				return errors.New("时长须为 1 至 3600 秒")
			}
		}
		for _, r := range b.Resolutions {
			if strings.TrimSpace(r) == "" {
				return errors.New("清晰度不能为空")
			}
		}
		if b.DrivingAudio && (b.MaxAudioBytes <= 0 || b.MinAudioDuration <= 0 || b.MaxAudioDuration < b.MinAudioDuration) {
			return errors.New("请配置音频大小和时长限制")
		}
	case Speech:
		if b.MaxTextLength <= 0 || len(b.Voices) == 0 {
			return errors.New("请配置音色、语言和文字长度上限")
		}
		for _, v := range b.Voices {
			if v.ID == "" || len(v.Languages) == 0 {
				return errors.New("音色标识和语言不能为空")
			}
		}
	}
	return nil
}

func (m Model) Resolve(operation Capability) OperationStatus {
	b, exists := m.Operations[operation]
	if operation == Image && exists {
		b = b.ResolvedImageBinding()
	}
	result := OperationStatus{Binding: b}
	if !m.Enabled {
		result.Code = "disabled"
		result.Reason = "模型未启用"
		return result
	}
	if !slices.Contains(m.Capabilities, operation) {
		result.Code = "operation_disabled"
		result.Reason = "未声明此能力"
		return result
	}
	if !exists {
		result.Code = "configuration_missing"
		result.Reason = "请在模型管理配置协议与参数"
		return result
	}
	if err := b.Validate(operation); err != nil {
		result.Code = "invalid_configuration"
		if strings.HasPrefix(err.Error(), "protocol_unsupported") {
			result.Code = "protocol_unsupported"
		}
		result.Reason = err.Error()
		return result
	}
	result.Available = true
	result.Code = "configured"
	return result
}

func (m Model) InputCapability() (InputCapability, bool) {
	c := InputCapability{ModelID: m.ID, Version: fmt.Sprintf("model-config.%d", m.Revision), Operations: map[Capability]OperationStatus{}, Voices: []Voice{}, Durations: []int{}, Resolutions: []string{}}
	any := false
	for _, operation := range []Capability{Story, Image, Video, Speech} {
		s := m.Resolve(operation)
		c.Operations[operation] = s
		if !s.Available {
			continue
		}
		any = true
		b := s.Binding
		switch operation {
		case Image:
			c.ReferenceImages = b.MaxReferences
			c.ImageRatios = b.Ratios
		case Speech:
			c.Speech = true
			c.Voices = b.Voices
		case Video:
			c.ReferenceVideo = b.SupportsVideoMode("reference_images")
			c.FirstFrame = b.SupportsVideoMode("first_frame")
			c.ReferenceImages = b.MaxReferences
			c.Durations = b.Durations
			c.Resolutions = b.Resolutions
			c.Ratios = b.Ratios
		}
	}
	return c, any
}

// VideoCapability preserves the single-mode caller contract. Multi-mode calls must select a mode.
func (m Model) VideoCapability() (VideoCapability, bool) {
	c, err := m.VideoCapabilityFor("")
	return c, err == nil
}

func (m Model) VideoCapabilityFor(mode string) (VideoCapability, error) {
	status := m.Resolve(Video)
	if !status.Available {
		return VideoCapability{}, errors.New(status.Reason)
	}
	b := status.Binding
	selected, err := b.SelectVideoMode(mode)
	if err != nil {
		return VideoCapability{}, err
	}
	media := map[string]int{}
	if selected == "reference_images" {
		media["reference_image"] = b.MaxReferences
	} else {
		media["first_frame"] = 1
		if b.LastFrame {
			media["last_frame"] = 1
		}
		if b.DrivingAudio {
			media["driving_audio"] = 1
		}
	}
	return VideoCapability{ModelPrefix: b.Protocol, InputMode: selected, Durations: b.Durations, MinDuration: slices.Min(b.Durations), MaxDuration: slices.Max(b.Durations), Resolutions: b.Resolutions, AllowedMedia: media, MaxImageBytes: b.MaxImageBytes, MaxAudioBytes: b.MaxAudioBytes, MinAudioDuration: b.MinAudioDuration, MaxAudioDuration: b.MaxAudioDuration}, nil
}

// VideoInputModes reads old single-mode configurations without changing stored snapshots.
func (b OperationBinding) VideoInputModes() []string {
	if b.InputModes != nil {
		return b.InputModes
	}
	if b.InputMode != "" {
		return []string{b.InputMode}
	}
	return nil
}
func (b OperationBinding) SupportsVideoMode(mode string) bool {
	return slices.Contains(b.VideoInputModes(), mode)
}
func (b OperationBinding) SelectVideoMode(mode string) (string, error) {
	modes := b.VideoInputModes()
	if mode == "" && len(modes) == 1 {
		return modes[0], nil
	}
	if !slices.Contains(modes, mode) {
		return "", errors.New("请选择模型支持的视频生成模式")
	}
	return mode, nil
}

func (m Model) Require(operation Capability, protocol string) error {
	s := m.Resolve(operation)
	if !s.Available {
		return errors.New(s.Code + ": " + s.Reason)
	}
	if s.Binding.Protocol != protocol {
		return ErrUnsupported
	}
	return nil
}

var ErrRevisionConflict = errors.New("模型配置已更新，请刷新后重试")

type OperationRepository interface {
	ConfigureOperations(context.Context, string, int64, map[Capability]OperationBinding, []Capability) (Model, error)
}

// SelectCapabilities records explicit user intent, including an empty selection.
// Clearing the last capability disables the model without deleting protocol settings.
func (m Model) SelectCapabilities(capabilities []Capability) (Model, error) {
	next := m
	next.Capabilities = []Capability{}
	for _, capability := range capabilities {
		switch capability {
		case Story, Image, Video, Speech:
		default:
			return m, fmt.Errorf("未知模型能力：%s", capability)
		}
		if !slices.Contains(next.Capabilities, capability) {
			next.Capabilities = append(next.Capabilities, capability)
		}
	}
	next.Supported = len(next.Capabilities) > 0
	if !next.Supported {
		next.Enabled = false
		next.Default = false
	}
	next.CapabilitiesConfigured = true
	next.Availability = nil
	return next, nil
}

// Configure returns a new aggregate revision without aliasing caller-owned maps.
func (m Model) Configure(expected int64, bindings map[Capability]OperationBinding, capabilities []Capability) (Model, error) {
	if m.Revision != expected {
		return m, ErrRevisionConflict
	}
	next := m
	if capabilities != nil {
		var err error
		next, err = m.SelectCapabilities(capabilities)
		if err != nil {
			return m, err
		}
	}
	next.Availability = nil
	next.Operations = map[Capability]OperationBinding{}
	for op, b := range bindings {
		var err error
		b, err = b.Compile(op)
		if err != nil {
			return m, fmt.Errorf("%s: %w", op, err)
		}
		b.Source = "manual"
		if b.EndpointMode == "default" {
			b.Endpoint = ""
		}
		next.Operations[op] = b
	}
	next.Revision++
	raw, err := json.Marshal(next)
	if err != nil {
		return m, err
	}
	var frozen Model
	err = json.Unmarshal(raw, &frozen)
	return frozen, err
}
