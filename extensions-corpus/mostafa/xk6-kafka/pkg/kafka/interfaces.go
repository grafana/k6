package kafka

type Serdes interface {
	Serialize(data any, schema *Schema) ([]byte, *Xk6KafkaError)
	Deserialize(data []byte, schema *Schema) (any, *Xk6KafkaError)
}

var TypesRegistry map[SchemaType]Serdes = map[SchemaType]Serdes{
	String:   &StringSerde{},
	Bytes:    &ByteArraySerde{},
	Json:     &JSONSerde{},
	Avro:     &AvroSerde{},
	Protobuf: &ProtobufSerde{},
}

func GetSerdes(schemaType SchemaType) (Serdes, *Xk6KafkaError) {
	if serdes, ok := TypesRegistry[schemaType]; ok {
		return serdes, nil
	}
	return nil, ErrUnknownSerdesType
}
