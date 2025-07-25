// Code generated by go-swagger; DO NOT EDIT.

package user

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/go-swagger/go-swagger/examples/generated/models"
)

// NewCreateUsersWithArrayInputParams creates a new CreateUsersWithArrayInputParams object
//
// There are no default values defined in the spec.
func NewCreateUsersWithArrayInputParams() CreateUsersWithArrayInputParams {

	return CreateUsersWithArrayInputParams{}
}

// CreateUsersWithArrayInputParams contains all the bound params for the create users with array input operation
// typically these are obtained from a http.Request
//
// swagger:parameters createUsersWithArrayInput
type CreateUsersWithArrayInputParams struct {
	// HTTP Request Object
	HTTPRequest *http.Request `json:"-"`

	/*List of user object
	  In: body
	*/
	Body []*models.User
}

// BindRequest both binds and validates a request, it assumes that complex things implement a Validatable(strfmt.Registry) error interface
// for simple values it will use straight method calls.
//
// To ensure default values, the struct must have been initialized with NewCreateUsersWithArrayInputParams() beforehand.
func (o *CreateUsersWithArrayInputParams) BindRequest(r *http.Request, route *middleware.MatchedRoute) error {
	var res []error

	o.HTTPRequest = r

	if runtime.HasBody(r) {
		defer func() {
			_ = r.Body.Close()
		}()
		var body []*models.User
		if err := route.Consumer.Consume(r.Body, &body); err != nil {
			res = append(res, errors.NewParseError("body", "body", "", err))
		} else {

			// validate array of body objects
			for i := range body {
				if body[i] == nil {
					continue
				}
				if err := body[i].Validate(route.Formats); err != nil {
					res = append(res, err)
					break
				}
			}

			if len(res) == 0 {
				o.Body = body
			}
		}
	}
	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
