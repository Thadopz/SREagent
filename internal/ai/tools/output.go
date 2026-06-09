package tools

import "SREagent/internal/ai/evidence"

type ToolOutput struct {
	Success          bool   `json:"success"`
	Status           string `json:"status,omitempty"`
	EvidenceID       string `json:"evidence_id,omitempty"`
	FailedQueryID    string `json:"failed_query_id,omitempty"`
	CanUseAsEvidence bool   `json:"can_use_as_evidence,omitempty"`
	DataStatus       string `json:"data_status,omitempty"`
	Data             any    `json:"data,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
	Message          string `json:"message,omitempty"`
	Summary          string `json:"summary,omitempty"`
}

func marshalToolOutput(output ToolOutput) string {
	return evidence.Marshal(evidence.Envelope{
		Success:          output.Success,
		Status:           output.Status,
		EvidenceID:       output.EvidenceID,
		FailedQueryID:    output.FailedQueryID,
		CanUseAsEvidence: output.CanUseAsEvidence,
		DataStatus:       output.DataStatus,
		Data:             output.Data,
		Error:            output.Error,
		ErrorCode:        output.ErrorCode,
		Message:          output.Message,
		Summary:          output.Summary,
	})
}

func marshalToolData(data any, message string) string {
	return evidence.Marshal(evidence.NewData(data, message))
}

func marshalToolError(err error, message string) string {
	return evidence.Marshal(evidence.NewError(err, message))
}
