// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/go-swagger/go-swagger/examples/composed-auth/models"
)

// GetOrderOKCode is the HTTP code returned for type GetOrderOK
const GetOrderOKCode int = 200

/*
GetOrderOK content of an order

swagger:response getOrderOK
*/
type GetOrderOK struct {

	/*
	  In: Body
	*/
	Payload *models.Order `json:"body,omitempty"`
}

// NewGetOrderOK creates GetOrderOK with default headers values
func NewGetOrderOK() *GetOrderOK {

	return &GetOrderOK{}
}

// WithPayload adds the payload to the get order o k response
func (o *GetOrderOK) WithPayload(payload *models.Order) *GetOrderOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the get order o k response
func (o *GetOrderOK) SetPayload(payload *models.Order) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *GetOrderOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// GetOrderUnauthorizedCode is the HTTP code returned for type GetOrderUnauthorized
const GetOrderUnauthorizedCode int = 401

/*
GetOrderUnauthorized unauthorized access for a lack of authentication

swagger:response getOrderUnauthorized
*/
type GetOrderUnauthorized struct {
}

// NewGetOrderUnauthorized creates GetOrderUnauthorized with default headers values
func NewGetOrderUnauthorized() *GetOrderUnauthorized {

	return &GetOrderUnauthorized{}
}

// WriteResponse to the client
func (o *GetOrderUnauthorized) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) // Remove Content-Type on empty responses

	rw.WriteHeader(401)
}

// GetOrderForbiddenCode is the HTTP code returned for type GetOrderForbidden
const GetOrderForbiddenCode int = 403

/*
GetOrderForbidden forbidden access for a lack of sufficient privileges

swagger:response getOrderForbidden
*/
type GetOrderForbidden struct {
}

// NewGetOrderForbidden creates GetOrderForbidden with default headers values
func NewGetOrderForbidden() *GetOrderForbidden {

	return &GetOrderForbidden{}
}

// WriteResponse to the client
func (o *GetOrderForbidden) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) // Remove Content-Type on empty responses

	rw.WriteHeader(403)
}

/*
GetOrderDefault other error response

swagger:response getOrderDefault
*/
type GetOrderDefault struct {
	_statusCode int

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewGetOrderDefault creates GetOrderDefault with default headers values
func NewGetOrderDefault(code int) *GetOrderDefault {
	if code <= 0 {
		code = 500
	}

	return &GetOrderDefault{
		_statusCode: code,
	}
}

// WithStatusCode adds the status to the get order default response
func (o *GetOrderDefault) WithStatusCode(code int) *GetOrderDefault {
	o._statusCode = code
	return o
}

// SetStatusCode sets the status to the get order default response
func (o *GetOrderDefault) SetStatusCode(code int) {
	o._statusCode = code
}

// WithPayload adds the payload to the get order default response
func (o *GetOrderDefault) WithPayload(payload *models.Error) *GetOrderDefault {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the get order default response
func (o *GetOrderDefault) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *GetOrderDefault) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(o._statusCode)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}
