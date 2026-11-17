package workspace

import (
	"strings"
)

func (s *Service) expandRepositoryInstruction(instruction string) (string, error) {
	if instruction == "" {
		return "", nil
	}
	var commands []QuickCommand
	if s.commands != nil {
		commands = s.commands()
	}
	expanded, missing := expandInstruction(instruction, commands)
	if len(missing) > 0 {
		return "", problemf(ErrInvalid, "初始化指令引用了不存在的快捷指令：%s", strings.Join(missing, "、"))
	}
	return expanded, nil
}
