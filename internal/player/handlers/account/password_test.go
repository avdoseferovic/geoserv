package account

import (
	"strings"
	"testing"
)

func TestHashAndValidatePassword(t *testing.T) {
	t.Parallel()
	hash := HashPassword("testuser", "testpass")
	if hash == "" {
		t.Fatal("hash should not be empty")
	}

	if !ValidatePassword("testuser", "testpass", hash) {
		t.Fatal("password should validate")
	}

	if ValidatePassword("testuser", "wrongpass", hash) {
		t.Fatal("wrong password should not validate")
	}

	if ValidatePassword("wronguser", "testpass", hash) {
		t.Fatal("wrong username should not validate")
	}
}

func TestHashPassword_Format(t *testing.T) {
	t.Parallel()
	hash := HashPassword("user", "pass")

	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash should start with $argon2id$v=19$, got %q", hash[:30])
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("hash should have 6 parts separated by $, got %d", len(parts))
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	t.Parallel()
	h1 := HashPassword("user", "pass")
	h2 := HashPassword("user", "pass")

	if h1 == h2 {
		t.Error("two hashes of the same password should differ (different salts)")
	}

	// But both should validate
	if !ValidatePassword("user", "pass", h1) {
		t.Error("h1 should validate")
	}
	if !ValidatePassword("user", "pass", h2) {
		t.Error("h2 should validate")
	}
}

func TestHashPassword_CaseInsensitiveUsername(t *testing.T) {
	t.Parallel()
	hash := HashPassword("TestUser", "mypass")

	if !ValidatePassword("testuser", "mypass", hash) {
		t.Error("lowercase username should validate")
	}
	if !ValidatePassword("TESTUSER", "mypass", hash) {
		t.Error("uppercase username should validate")
	}
	if !ValidatePassword("TestUser", "mypass", hash) {
		t.Error("mixed case username should validate")
	}
}

func TestHashPassword_CaseSensitivePassword(t *testing.T) {
	t.Parallel()
	hash := HashPassword("user", "MyPass")

	if !ValidatePassword("user", "MyPass", hash) {
		t.Error("exact password should validate")
	}
	if ValidatePassword("user", "mypass", hash) {
		t.Error("lowercase password should NOT validate")
	}
	if ValidatePassword("user", "MYPASS", hash) {
		t.Error("uppercase password should NOT validate")
	}
}

func TestValidatePassword_MalformedHash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		hash string
	}{
		{"empty string", ""},
		{"random garbage", "not-a-hash"},
		{"too few parts", "$argon2id$v=19$m=65536"},
		{"bad params", "$argon2id$v=19$garbage$c2FsdA$aGFzaA"},
		{"bad salt base64", "$argon2id$v=19$m=65536,t=1,p=4$!!!invalid!!!$aGFzaA"},
		{"bad hash base64", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$!!!invalid!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if ValidatePassword("user", "pass", tt.hash) {
				t.Errorf("ValidatePassword should return false for malformed hash %q", tt.hash)
			}
		})
	}
}

func TestValidatePassword_EmptyInputs(t *testing.T) {
	t.Parallel()
	// Empty password should still work (just hash empty string)
	hash := HashPassword("user", "")
	if !ValidatePassword("user", "", hash) {
		t.Error("empty password should validate against its own hash")
	}
	if ValidatePassword("user", "notempty", hash) {
		t.Error("non-empty password should not validate against empty password hash")
	}
}
