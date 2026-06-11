package mail

import "testing"

func TestNewDisabledWhenUnconfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "empty", cfg: Config{}, want: false},
		{name: "host only", cfg: Config{Host: "smtp.example.com"}, want: false},
		{name: "from only", cfg: Config{From: "x@example.com"}, want: false},
		{
			name: "host and from",
			cfg:  Config{Host: "smtp.example.com", From: "x@example.com", Port: 587},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.cfg).Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisabledSendFails(t *testing.T) {
	if err := (Disabled{}).Send("a@b.c", "s", "b"); err == nil {
		t.Error("Disabled.Send should error")
	}
}
