
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/encoding/protojson"
)

// IssueKind is the kind of a check issue.
type IssueKind string

const (
	IssueKindLoad          IssueKind = "load"
	IssueKindCheck         IssueKind = "check"
	IssueKindCompatibility IssueKind = "compatibility"
)

// Issue represents a single structured check error.
type Issue struct {
	Kind      IssueKind                   `json:"kind"`
	Message   string                      `json:"message"`
	Workbook  *tableaupb.WorkbookOptions  `json:"workbook,omitempty"`
	Worksheet *tableaupb.WorksheetOptions `json:"worksheet,omitempty"`
}

// Error returns the issue as a human-readable string.
func (i *Issue) Error() string {
	return fmt.Sprintf("error: workbook %s, worksheet %s, %s",
		i.Workbook.GetName(),
		i.Worksheet.GetName(),
		i.Message)
}

// MarshalJSON uses protojson for Workbook/Worksheet fields to emit correct proto field names.
func (i *Issue) MarshalJSON() ([]byte, error) {
	marshaler := protojson.MarshalOptions{}
	out := struct {
		Kind      IssueKind       `json:"kind"`
		Message   string          `json:"message"`
		Workbook  json.RawMessage `json:"workbook,omitempty"`
		Worksheet json.RawMessage `json:"worksheet,omitempty"`
	}{
		Kind:    i.Kind,
		Message: i.Message,
	}
	if i.Workbook != nil {
		b, err := marshaler.Marshal(i.Workbook)
		if err != nil {
			return nil, err
		}
		out.Workbook = json.RawMessage(b)
	}
	if i.Worksheet != nil {
		b, err := marshaler.Marshal(i.Worksheet)
		if err != nil {
			return nil, err
		}
		out.Worksheet = json.RawMessage(b)
	}
	return json.Marshal(out)
}

// ErrorFormat is a function type that formats a CheckError into a string.
type ErrorFormat func(*CheckError) string

// ErrorFormatText formats issues as human-readable text lines (default).
var ErrorFormatText ErrorFormat = func(e *CheckError) string {
	msgs := make([]string, len(e.Issues))
	for i, issue := range e.Issues {
		msgs[i] = fmt.Sprintf("error: workbook %s, worksheet %s",
			issue.Workbook.GetName(),
			issue.Worksheet.GetName())
	}
	return strings.Join(msgs, "\n")
}

// ErrorFormatJSON formats the CheckError as a JSON object.
var ErrorFormatJSON ErrorFormat = func(e *CheckError) string {
	b, err := json.Marshal(e)
	if err != nil {
		log.Errorf("failed to marshal CheckError to JSON: %+v", err)
		return ""
	}
	return string(b)
}

// CheckError is the error type returned by Check and CheckCompatibility.
type CheckError struct {
	Issues []*Issue `json:"issues"`
	format ErrorFormat
}

// Error formats the result using the configured ErrorFormat.
// Falls back to ErrorFormatText if format is nil.
func (e *CheckError) Error() string {
	if e.format == nil {
		return ErrorFormatText(e)
	}
	return e.format(e)
}
