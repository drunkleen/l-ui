package service

import (
	"errors"

	"github.com/drunkleen/l-ui/internal/util/common"
)

const (
	NodeErrAuth     = "node_auth"
	NodeErrConfig   = "node_config"
	NodeErrService  = "node_service"
	NodeErrVersion  = "node_version"
	NodeErrNotFound = "node_not_found"
	NodeErrDisabled = "node_disabled"
)

func codedNodeErr(code string, msg string) error {
	return common.NewCodedError(code, errors.New(msg))
}

func nodeNotFoundErr() error          { return codedNodeErr(NodeErrNotFound, "node not found") }
func nodeDisabledErr() error          { return codedNodeErr(NodeErrDisabled, "node is disabled") }
func nodeConfigErr(msg string) error  { return common.NewCodedError(NodeErrConfig, errors.New(msg)) }
func nodeServiceErr(msg string) error { return common.NewCodedError(NodeErrService, errors.New(msg)) }
func nodeVersionErr(msg string) error { return common.NewCodedError(NodeErrVersion, errors.New(msg)) }
func nodeAuthErr(msg string) error    { return common.NewCodedError(NodeErrAuth, errors.New(msg)) }
