/*
Copyright 2025 Red Hat, Inc.
This file contains code generated or modified with AI assistance.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vsphere

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestLoadCredentialsFromSecret tests loading credentials from Kubernetes Secret
func TestLoadCredentialsFromSecret(t *testing.T) {
	tests := []struct {
		name        string
		secret      *corev1.Secret
		expectError bool
		validate    func(*testing.T, *Credentials)
	}{
		{
			name: "Valid credentials with all fields",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"vCenter":       []byte("vcenter.example.com"),
					"username":      []byte("administrator@vsphere.local"),
					"password":      []byte("secret-password"),
					"insecure":      []byte("true"),
					"cacertificate": []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
				},
			},
			expectError: false,
			validate: func(t *testing.T, creds *Credentials) {
				if creds.Server != "vcenter.example.com" {
					t.Errorf("Expected server vcenter.example.com, got %s", creds.Server)
				}
				if creds.Username != "administrator@vsphere.local" {
					t.Errorf("Expected username administrator@vsphere.local, got %s", creds.Username)
				}
				if creds.Password != "secret-password" {
					t.Errorf("Expected password secret-password, got %s", creds.Password)
				}
				if !creds.Insecure {
					t.Error("Expected insecure to be true")
				}
				if creds.CACertificate == "" {
					t.Error("Expected CA certificate to be set")
				}
			},
		},
		{
			name: "Valid credentials without optional fields",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"vCenter":  []byte("vcenter.example.com"),
					"username": []byte("admin"),
					"password": []byte("pass"),
				},
			},
			expectError: false,
			validate: func(t *testing.T, creds *Credentials) {
				if creds.Server != "vcenter.example.com" {
					t.Errorf("Expected server vcenter.example.com, got %s", creds.Server)
				}
				if creds.Insecure {
					t.Error("Expected insecure to be false by default")
				}
			},
		},
		{
			name: "Missing vCenter field",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("pass"),
				},
			},
			expectError: true,
		},
		{
			name: "Missing username field",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"vCenter":  []byte("vcenter.example.com"),
					"password": []byte("pass"),
				},
			},
			expectError: true,
		},
		{
			name: "Missing password field",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"vCenter":  []byte("vcenter.example.com"),
					"username": []byte("admin"),
				},
			},
			expectError: true,
		},
		{
			name: "Insecure set to false",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"vCenter":  []byte("vcenter.example.com"),
					"username": []byte("admin"),
					"password": []byte("pass"),
					"insecure": []byte("false"),
				},
			},
			expectError: false,
			validate: func(t *testing.T, creds *Credentials) {
				if creds.Insecure {
					t.Error("Expected insecure to be false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := runtime.NewScheme()
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.secret).
				Build()

			ctx := context.Background()
			creds, err := LoadCredentialsFromSecret(ctx, fakeClient, "default", "vsphere-creds")

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if creds == nil {
				t.Fatal("Expected credentials but got nil")
			}

			if tt.validate != nil {
				tt.validate(t, creds)
			}
		})
	}
}

// TestLoadCredentialsFromSecretNotFound tests error when secret doesn't exist
func TestLoadCredentialsFromSecretNotFound(t *testing.T) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	ctx := context.Background()
	_, err := LoadCredentialsFromSecret(ctx, fakeClient, "default", "nonexistent-secret")

	if err == nil {
		t.Fatal("Expected error when secret doesn't exist")
	}
}

// TestCredentialsValidation tests credential field validation
func TestCredentialsValidation(t *testing.T) {
	tests := []struct {
		name  string
		creds *Credentials
		valid bool
	}{
		{
			name: "All required fields present",
			creds: &Credentials{
				Server:   "vcenter.example.com",
				Username: "admin",
				Password: "password",
			},
			valid: true,
		},
		{
			name: "Empty server",
			creds: &Credentials{
				Server:   "",
				Username: "admin",
				Password: "password",
			},
			valid: false,
		},
		{
			name: "Empty username",
			creds: &Credentials{
				Server:   "vcenter.example.com",
				Username: "",
				Password: "password",
			},
			valid: false,
		},
		{
			name: "Empty password",
			creds: &Credentials{
				Server:   "vcenter.example.com",
				Username: "admin",
				Password: "",
			},
			valid: false,
		},
		{
			name: "All fields empty",
			creds: &Credentials{
				Server:   "",
				Username: "",
				Password: "",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.creds.Server != "" && tt.creds.Username != "" && tt.creds.Password != ""

			if isValid != tt.valid {
				t.Errorf("Expected valid=%v, got %v", tt.valid, isValid)
			}
		})
	}
}

// TestCredentialsStructure tests the Credentials struct
func TestCredentialsStructure(t *testing.T) {
	creds := &Credentials{
		Server:        "vcenter.example.com",
		Username:      "admin@vsphere.local",
		Password:      "MySecretPassword123",
		Insecure:      true,
		CACertificate: "-----BEGIN CERTIFICATE-----",
	}

	if creds.Server == "" {
		t.Error("Server should be set")
	}
	if creds.Username == "" {
		t.Error("Username should be set")
	}
	if creds.Password == "" {
		t.Error("Password should be set")
	}
	if !creds.Insecure {
		t.Error("Insecure should be true")
	}
	if creds.CACertificate == "" {
		t.Error("CACertificate should be set")
	}
}

// TestSecretDataTypes tests different data formats in secret
func TestSecretDataTypes(t *testing.T) {
	tests := []struct {
		name       string
		secretData map[string][]byte
		expectOK   bool
	}{
		{
			name: "String values as bytes",
			secretData: map[string][]byte{
				"vCenter":  []byte("vcenter.example.com"),
				"username": []byte("admin"),
				"password": []byte("pass"),
			},
			expectOK: true,
		},
		{
			name: "Empty byte arrays",
			secretData: map[string][]byte{
				"vCenter":  []byte(""),
				"username": []byte(""),
				"password": []byte(""),
			},
			expectOK: false,
		},
		{
			name: "Unicode characters",
			secretData: map[string][]byte{
				"vCenter":  []byte("vcenter.example.com"),
				"username": []byte("admin@vsphere.local"),
				"password": []byte("пароль123"),
			},
			expectOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-creds",
					Namespace: "default",
				},
				Data: tt.secretData,
			}

			s := runtime.NewScheme()
			_ = scheme.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(secret).
				Build()

			ctx := context.Background()
			creds, err := LoadCredentialsFromSecret(ctx, fakeClient, "default", "vsphere-creds")

			if tt.expectOK {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if creds == nil {
					t.Error("Expected credentials, got nil")
				}
			} else {
				if err == nil {
					t.Error("Expected error for invalid credentials")
				}
			}
		})
	}
}
