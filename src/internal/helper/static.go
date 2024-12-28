package helper

const ErrorPofileSpecification = "must specify --profile"
const SessionDuration = "43200" //12 hours
const (
	ISO8601WithOffset    = "2006-01-02T15:04:05-0700"
	ISO8601WithFixedZone = "2006-01-02T15:04:05Z0700"
	ISO8601WithUTC       = "2006-01-02T15:04:05UTC"
)

var TimeLayouts = []string{
	ISO8601WithOffset,
	ISO8601WithFixedZone,
	ISO8601WithUTC,
}
