package service

import (
	olympus "github.com/ihezebin/olympus/httpserver"
)

func BadRequest(err error) error {
	return olympus.NewError(olympus.CodeBadRequest, err.Error())
}
