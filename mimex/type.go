package mimex

import "fmt"

// Type is the MIME type of content/media.
type Type string

const (
	// text

	TextPlain Type = "text/plain"
	TextHTML  Type = "text/html"

	// application

	ApplicationJSON            Type = "application/json"
	ApplicationXML             Type = "application/xml"
	ApplicationFormURLEncoded  Type = "application/x-www-form-urlencoded"
	ApplicationOctetStream     Type = "application/octet-stream"
	ApplicationProtobuf        Type = "application/protobuf"
	ApplicationMsExcel         Type = "application/vnd.ms-excel"
	ApplicationMsExcelAlt      Type = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	ApplicationMsWord          Type = "application/msword"
	ApplicationMsWordAlt       Type = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	ApplicationMsPowerPoint    Type = "application/vnd.ms-powerpoint"
	ApplicationMsPowerPointAlt Type = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	ApplicationPDF             Type = "application/pdf"

	// audio

	AudioAAC Type = "audio/aac"
	AudioAMR Type = "audio/amr"
	AudioMP3 Type = "audio/mp3"
	AudioMP4 Type = "audio/mp4"
	AudioOGG Type = "audio/ogg"

	// image

	ImageWebp Type = "image/webp"
	ImageJPEG Type = "image/jpeg"
	ImagePNG  Type = "image/png"

	// video

	Video3GPP Type = "video/3gpp"
	VideoMP4  Type = "video/mp4"
)

var _ fmt.Stringer = Type("")

func (t Type) String() string {
	return string(t)
}

type Codec string

const (
	CodecOpus Codec = "opus"
)
