package proc

const (
	// This URL will be polled to fetch the UNL validators.
	xrplFoundationURL = "https://unl.xrplf.org"
)

// unlResponse is the schema of the response received from the [xrplFoundationURL].
type unlResponse struct {
	Blob      string `json:"blob"`
	Manifest  string `json:"manifest"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
	Version   int    `json:"version"`
}

// unlValidatorInfo is the schema of the .Blob field from the UNL response once decoded.
type unlValidatorInfo struct {
	Sequence   int64 `json:"sequence"`
	Expiration int64 `json:"expiration"`
	Validators []struct {
		ValidationPublicKey string `json:"validation_public_key"`
		Manifest            string `json:"manifest"`
	} `json:"validators"`
}
