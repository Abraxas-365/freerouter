package access

import "testing"

func TestCreateUser_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateUser
		wantErr bool
	}{
		{"valid", CreateUser{Email: "a@b.com", Name: "Alice", Password: "12345678"}, false},
		{"missing email", CreateUser{Name: "Alice", Password: "12345678"}, true},
		{"missing name", CreateUser{Email: "a@b.com", Password: "12345678"}, true},
		{"short password", CreateUser{Email: "a@b.com", Name: "Alice", Password: "1234567"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateRole
		wantErr bool
	}{
		{"valid", CreateRole{Name: "admin", Permissions: []string{"freerouter:gateway:invoke"}}, false},
		{"missing name", CreateRole{Permissions: []string{"freerouter:gateway:invoke"}}, true},
		{"no permissions", CreateRole{Name: "empty"}, true},
		{"unknown permission", CreateRole{Name: "bad", Permissions: []string{"unknown:perm"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   UpdateRole
		wantErr bool
	}{
		{"valid", UpdateRole{Name: "admin", Permissions: []string{"freerouter:metrics:read"}}, false},
		{"missing name", UpdateRole{Permissions: []string{"freerouter:metrics:read"}}, true},
		{"no permissions", UpdateRole{Name: "empty"}, true},
		{"unknown permission", UpdateRole{Name: "bad", Permissions: []string{"nope"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAssignRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   AssignRole
		wantErr bool
	}{
		{"valid", AssignRole{UserID: "u1", RoleID: "r1"}, false},
		{"missing user", AssignRole{RoleID: "r1"}, true},
		{"missing role", AssignRole{UserID: "u1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
