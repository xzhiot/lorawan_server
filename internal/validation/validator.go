package validation

import (
    "fmt"
    "reflect"
    "strings"
)

// Validator validates structs
type Validator struct{}

// NewValidator creates a new validator
func NewValidator() *Validator {
    return &Validator{}
}

// Validate validates a struct
func (v *Validator) Validate(s interface{}) error {
    // Simple validation implementation
    // In production, use a library like github.com/go-playground/validator
    
    val := reflect.ValueOf(s)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }
    
    if val.Kind() != reflect.Struct {
        return fmt.Errorf("validate expects a struct")
    }
    
    typ := val.Type()
    
    for i := 0; i < val.NumField(); i++ {
        field := val.Field(i)
        fieldType := typ.Field(i)
        tag := fieldType.Tag.Get("validate")
        
        if tag == "" {
            continue
        }
        
        if err := v.validateField(field, fieldType, tag); err != nil {
            return fmt.Errorf("%s: %w", fieldType.Name, err)
        }
    }
    
    return nil
}

// validateField validates a single field
func (v *Validator) validateField(field reflect.Value, fieldType reflect.StructField, tag string) error {
    rules := strings.Split(tag, ",")
    
    for _, rule := range rules {
        parts := strings.SplitN(rule, "=", 2)
        ruleName := parts[0]
        
        switch ruleName {
        case "required":
            if field.IsZero() {
                return fmt.Errorf("field is required")
            }
            
        case "email":
            if field.Kind() == reflect.String {
                email := field.String()
                if !strings.Contains(email, "@") {
                    return fmt.Errorf("invalid email format")
                }
            }
            
        case "min":
            if len(parts) < 2 {
                continue
            }
            // Simplified min validation
            if field.Kind() == reflect.String && len(field.String()) < 3 {
                return fmt.Errorf("minimum length is 3")
            }
            
        case "max":
            // Similar to min
            
        case "len":
            // Validate exact length
        }
    }
    
    return nil
}
