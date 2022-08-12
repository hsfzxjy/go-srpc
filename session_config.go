package srpc

import "time"

type SessionConfig struct {
	BufferCapacity int
	ClientTimeout  time.Duration
	KeepAlive      time.Duration
}

func (cfg *SessionConfig) validate() bool {
	return cfg.BufferCapacity >= 0 &&
		cfg.ClientTimeout > 0 &&
		cfg.KeepAlive > 0
}

func (cfg *SessionConfig) copyFrom(src *SessionConfig, strict bool) bool {
	if src == nil {
		return false
	}
	if strict && !src.validate() {
		return false
	}

	if src.BufferCapacity >= 0 {
		cfg.BufferCapacity = src.BufferCapacity
	}

	if src.ClientTimeout > 0 {
		cfg.ClientTimeout = src.ClientTimeout
	}

	if src.KeepAlive > 0 {
		cfg.KeepAlive = src.KeepAlive
	}

	return true
}

func mergeConfig(cfg *SessionConfig) (ret *SessionConfig) {
	ret = &SessionConfig{}
	ret.copyFrom(&defaultSessionConfig, false)
	ret.copyFrom(cfg, false)
	return
}
