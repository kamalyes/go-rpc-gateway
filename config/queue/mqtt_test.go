package queue

import "testing"

func TestMQTTValidation(t *testing.T) {
	for _, tc := range []struct {
		cfg       *Mqtt
		wantError bool
	}{
		{nil, false}, {&Mqtt{}, false}, {Default(), false},
		{&Mqtt{Enabled: true}, true}, {Default().WithEnabled(true), false},
	} {
		if err := tc.cfg.Validate(); (err != nil) != tc.wantError {
			t.Fatalf("config=%+v, err=%v", tc.cfg, err)
		}
	}
}
