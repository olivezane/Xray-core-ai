package serial

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// SettingsNamer is how a settings type that is not a protobuf message says what
// the API calls it. A protobuf message already carries its own name, so only a
// configuration that is a plain Go value needs to say one.
type SettingsNamer interface {
	SettingsName() string
}

// NameOf is what the API calls a setting: the name a typed message carries, the
// name a plain Go value says for itself, or the name protobuf gives a message.
func NameOf(settings any) string {
	if typed, ok := settings.(*TypedMessage); ok {
		if typed == nil {
			return ""
		}
		return typed.Type
	}
	if named, ok := settings.(SettingsNamer); ok {
		return named.SettingsName()
	}
	if message, ok := settings.(proto.Message); ok {
		return GetMessageType(message)
	}
	return ""
}

// ToTypedMessage converts a setting into TypedMessage: a protobuf message by the
// name and encoding protobuf gives it, a plain Go value as JSON under the name it
// says for itself.
func ToTypedMessage(message any) *TypedMessage {
	if message == nil {
		return nil
	}
	// A value that is already carried this way is not carried twice.
	if typed, ok := message.(*TypedMessage); ok {
		return typed
	}
	if m, ok := message.(proto.Message); ok {
		settings, _ := proto.Marshal(m)
		return &TypedMessage{
			Type:  GetMessageType(m),
			Value: settings,
		}
	}
	named, ok := message.(SettingsNamer)
	if !ok {
		return nil
	}
	settings, err := json.Marshal(message)
	if err != nil {
		return nil
	}
	return &TypedMessage{
		Type:  named.SettingsName(),
		Value: settings,
	}
}

// GetMessageType returns the name of this proto Message.
func GetMessageType(message proto.Message) string {
	return string(message.ProtoReflect().Descriptor().FullName())
}

// GetInstance creates a new instance of the message with messageType.
func GetInstance(messageType string) (interface{}, error) {
	messageTypeDescriptor := protoreflect.FullName(messageType)
	mType, err := protoregistry.GlobalTypes.FindMessageByName(messageTypeDescriptor)
	if err != nil {
		return nil, err
	}
	return mType.New().Interface(), nil
}

// GetInstance converts current TypedMessage into a proto Message.
func (v *TypedMessage) GetInstance() (proto.Message, error) {
	instance, err := GetInstance(v.Type)
	if err != nil {
		return nil, err
	}
	protoMessage := instance.(proto.Message)
	if err := proto.Unmarshal(v.Value, protoMessage); err != nil {
		return nil, err
	}
	return protoMessage, nil
}
