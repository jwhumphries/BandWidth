package auth

import "github.com/pquerna/otp/totp"

// TOTPKey is a newly generated TOTP enrollment.
type TOTPKey struct {
	Secret string // base32 secret for manual entry
	URL    string // otpauth:// URL for QR codes
}

// NewTOTPKey generates a TOTP secret for the given account name.
func NewTOTPKey(account string) (*TOTPKey, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "BandWidth",
		AccountName: account,
	})
	if err != nil {
		return nil, err
	}
	return &TOTPKey{Secret: key.Secret(), URL: key.URL()}, nil
}

// ValidateTOTP reports whether code is currently valid for secret.
func ValidateTOTP(code, secret string) bool {
	return totp.Validate(code, secret)
}
