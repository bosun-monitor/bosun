package ntlm

// NtlmAuthenticator defines interface to provide methods to get byte arrays required for NTLM authentication
type NtlmAuthenticator interface {
	GetNegotiateBytes() ([]byte, error)
	GetResponseBytes([]byte) ([]byte, error)
	ReleaseContext()
}
