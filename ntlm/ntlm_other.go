// +build !windows

package ntlm

// NTLM authentication is only currently implemented on Windows
func getDefaultCredentialsAuth() (NtlmAuthenticator, bool) {
	return nil, false
}
