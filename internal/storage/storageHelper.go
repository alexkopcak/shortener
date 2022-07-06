package storage

type (
	// short URL value and original URL value pairs
	ItemType struct {
		ShortURLValue string `json:"shortURLValue"`
		LongURLValue  string `json:"longURLValue"`
	}

	// short URL value and original URL value pairs
	UserExportType struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}

	// array of BatchRequest.
	BatchRequestArray []BatchRequest

	// Batch request struct to unmarshal json request.
	BatchRequest struct {
		CorrelationID string `json:"correlation_id"`
		OriginalURL   string `json:"original_url"`
		ShortURL      string `json:"-"`
	}

	// array of BatchResponse
	BatchResponseArray []BatchResponse

	// Batch response struct to marshal json and response.
	BatchResponse struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}
)
