package utils

import (
    "fmt"
    "regexp"
    "strings"
    "time"

    "github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
    validate = validator.New()
}

func ValidateStruct(s interface{}) error {
    return validate.Struct(s)
}

func ValidatePAN(pan string) bool {
    panRegex := regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]{1}$`)
    return panRegex.MatchString(pan)
}

func ValidateAadhaar(aadhaar string) bool {
    aadhaarRegex := regexp.MustCompile(`^[0-9]{12}$`)
    return aadhaarRegex.MatchString(aadhaar)
}

func ValidateEmail(email string) bool {
    emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
    return emailRegex.MatchString(email)
}

func ValidatePhone(phone string) bool {
    phoneRegex := regexp.MustCompile(`^[0-9]{10,15}$`)
    return phoneRegex.MatchString(phone)
}

func IsValidAge(dob time.Time) bool {
    age := time.Since(dob).Hours() / 24 / 365.25
    return age >= 18
}

func SanitizeString(input string) string {
    return strings.TrimSpace(input)
}

func FormatValidationError(err error) map[string]string {
    errors := make(map[string]string)
    
    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        for _, fieldError := range validationErrors {
            field := strings.ToLower(fieldError.Field())
            switch fieldError.Tag() {
            case "required":
                errors[field] = fmt.Sprintf("%s is required", field)
            case "email":
                errors[field] = "Invalid email format"
            case "min":
                errors[field] = fmt.Sprintf("%s must be at least %s characters", field, fieldError.Param())
            case "max":
                errors[field] = fmt.Sprintf("%s must be at most %s characters", field, fieldError.Param())
            case "len":
                errors[field] = fmt.Sprintf("%s must be exactly %s characters", field, fieldError.Param())
            default:
                errors[field] = fmt.Sprintf("%s is invalid", field)
            }
        }
    }
    
    return errors
}