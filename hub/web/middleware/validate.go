package middleware

import (
	"errors"
	"reflect"
	"strings"

	"github.com/drunkleen/l-ui/hub/web/entity"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func BindAndValidate[T any](c fiber.Ctx) (*T, bool) {
	var dst T
	if err := c.Bind().Body(&dst); err != nil {
		writeBindFailure(c, err)
		return nil, false
	}
	if err := validate.Struct(&dst); err != nil {
		writeBindFailure(c, err)
		return nil, false
	}
	return &dst, true
}

func BindAndValidateInto(c fiber.Ctx, dst any) bool {
	if err := c.Bind().Body(dst); err != nil {
		writeBindFailure(c, err)
		return false
	}
	if err := validate.Struct(dst); err != nil {
		writeBindFailure(c, err)
		return false
	}
	return true
}

func BindJSONAndValidate[T any](c fiber.Ctx) (*T, bool) {
	var dst T
	if err := c.Bind().JSON(&dst); err != nil {
		writeBindFailure(c, err)
		return nil, false
	}
	if err := validate.Struct(&dst); err != nil {
		writeBindFailure(c, err)
		return nil, false
	}
	return &dst, true
}

func BindJSONAndValidateInto(c fiber.Ctx, dst any) bool {
	if err := c.Bind().JSON(dst); err != nil {
		writeBindFailure(c, err)
		return false
	}
	if err := validate.Struct(dst); err != nil {
		writeBindFailure(c, err)
		return false
	}
	return true
}

type FieldIssue struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

type ValidationPayload struct {
	Issues  []FieldIssue `json:"issues"`
	Message string       `json:"message"`
}

func writeBindFailure(c fiber.Ctx, err error) {
	payload := ValidationPayload{Issues: []FieldIssue{}, Message: err.Error()}

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		payload.Issues = make([]FieldIssue, 0, len(ve))
		for _, fe := range ve {
			payload.Issues = append(payload.Issues, FieldIssue{
				Field:   fe.Field(),
				Rule:    fe.Tag(),
				Param:   fe.Param(),
				Message: fe.Error(),
			})
		}
	}

	c.Status(fiber.StatusOK).JSON(entity.Msg{
		Success: false,
		Msg:     "request body failed validation",
		Obj:     payload,
	})
}

func init() {
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}
