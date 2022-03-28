package storage

type (
	ItemType struct {
		ShortURLValue string `json:"shortURLValue"`
		LongURLValue  string `json:"longURLValue"`
	}

	UserExportType struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}

	BatchRequestArray []BatchRequest

	BatchRequest struct {
		CorrelationID string `json:"correlation_id"`
		OriginalURL   string `json:"original_url"`
	}

	BatchResponseArray []BatchResponse

	BatchResponse struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}
)
