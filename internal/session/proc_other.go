//go:build !linux && !darwin

package session

type processTable struct{}

func processes() processTable                            { return processTable{} }
func foregroundBlocked(_ int, _ processTable) bool       { return false }
func awaitingInput(_ int, _ processTable) bool           { return false }
func enrichTimer(_ int, state processState) processState { return state }
