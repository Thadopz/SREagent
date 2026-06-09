package evidence

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	StatusSuccess          = "success"
	StatusError            = "error"
	StatusTimeout          = "timeout"
	StatusPermissionDenied = "permission_denied"

	DataPresent = "present"
	DataEmpty   = "empty"
	DataUnknown = "unknown"
)

type Envelope struct {
	Success          bool   `json:"success"`
	Status           string `json:"status"`
	EvidenceID       string `json:"evidence_id,omitempty"`
	FailedQueryID    string `json:"failed_query_id,omitempty"`
	CanUseAsEvidence bool   `json:"can_use_as_evidence"`
	DataStatus       string `json:"data_status"`
	Data             any    `json:"data,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
	Message          string `json:"message,omitempty"`
	Summary          string `json:"summary,omitempty"`
}

type Parsed struct {
	Envelope Envelope
	IsJSON   bool
}

func NewData(data any, message string) Envelope {
	dataStatus := DataPresent
	if isEmptyData(data) {
		dataStatus = DataEmpty
	}
	return Envelope{
		Success:          true,
		Status:           StatusSuccess,
		EvidenceID:       newID("ev"),
		CanUseAsEvidence: true,
		DataStatus:       dataStatus,
		Data:             data,
		Message:          message,
		Summary:          message,
	}
}

func NewError(err error, message string) Envelope {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	status := StatusError
	code := "ERROR"
	lower := strings.ToLower(message + " " + errMsg)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") {
		status = StatusTimeout
		code = "TIMEOUT"
	}
	if strings.Contains(lower, "authorization denied") || strings.Contains(lower, "permission") {
		status = StatusPermissionDenied
		code = "PERMISSION_DENIED"
	}
	return Envelope{
		Success:          false,
		Status:           status,
		FailedQueryID:    newID("fq"),
		CanUseAsEvidence: false,
		DataStatus:       DataUnknown,
		Error:            errMsg,
		ErrorCode:        code,
		Message:          message,
		Summary:          message,
	}
}

func Marshal(env Envelope) string {
	normalize(&env)
	b, err := json.Marshal(env)
	if err != nil {
		fallback := NewError(err, "marshal tool output failed")
		b, _ = json.Marshal(fallback)
	}
	return string(b)
}

func Parse(raw string) Parsed {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Parsed{Envelope: NewError(nil, "empty tool response")}
	}
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return Parsed{
			IsJSON: false,
			Envelope: Envelope{
				Success:          true,
				Status:           StatusSuccess,
				CanUseAsEvidence: false,
				DataStatus:       DataUnknown,
				Message:          truncate(raw, 300),
				Summary:          truncate(raw, 300),
			},
		}
	}
	normalize(&env)
	return Parsed{Envelope: env, IsJSON: true}
}

func normalize(env *Envelope) {
	if env == nil {
		return
	}
	if strings.TrimSpace(env.Status) == "" {
		if env.Success {
			env.Status = StatusSuccess
		} else {
			env.Status = StatusError
		}
	}
	env.Status = strings.ToLower(strings.TrimSpace(env.Status))
	if env.Status == StatusSuccess {
		env.Success = true
		if strings.TrimSpace(env.DataStatus) == "" {
			if isEmptyData(env.Data) {
				env.DataStatus = DataEmpty
			} else {
				env.DataStatus = DataPresent
			}
		}
		if strings.TrimSpace(env.EvidenceID) == "" {
			env.CanUseAsEvidence = false
		}
	} else {
		env.Success = false
		env.CanUseAsEvidence = false
		env.EvidenceID = ""
		if strings.TrimSpace(env.FailedQueryID) == "" {
			env.FailedQueryID = newID("fq")
		}
		if strings.TrimSpace(env.DataStatus) == "" {
			env.DataStatus = DataUnknown
		}
	}
	if strings.TrimSpace(env.Summary) == "" {
		env.Summary = env.Message
	}
	if strings.TrimSpace(env.Summary) == "" && env.Error != "" {
		env.Summary = env.Error
	}
}

func isEmptyData(data any) bool {
	if data == nil {
		return true
	}
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Slice, reflect.Map, reflect.Array, reflect.String:
		return v.Len() == 0
	default:
		return false
	}
}

func newID(prefix string) string {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(bytes[:]))
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
