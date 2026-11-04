package users

import (
	"reflect"
	"testing"
)

func TestFrontendUserContainsOtpEnabled(t *testing.T) {
	structType := reflect.TypeOf(FrontendUser{})

	otpEnabledField, exists := structType.FieldByName("OtpEnabled")
	if !exists {
		t.Fatal("OtpEnabled field should exist on FrontendUser")
	}

	expectedTag := "otpEnabled"
	if otpEnabledField.Tag.Get("json") != expectedTag {
		t.Errorf("Expected JSON tag '%s', got '%s'", expectedTag, otpEnabledField.Tag.Get("json"))
	}

	if otpEnabledField.Type.Kind() != reflect.Bool {
		t.Errorf("Expected OtpEnabled to be bool, got %s", otpEnabledField.Type.Kind())
	}
}

func TestNonAdminEditableDoesNotDuplicateOtpEnabled(t *testing.T) {
	structType := reflect.TypeOf(NonAdminEditable{})
	if _, exists := structType.FieldByName("OtpEnabled"); exists {
		t.Fatal("OtpEnabled should not be duplicated on NonAdminEditable")
	}
}
