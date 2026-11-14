package editor

import (
	"strings"
	"testing"
)

func TestEnsureReactImportNew(t *testing.T) {
	buf := NewBuffer("export const MyComp = () => <div>Hello</div>;\n")

	applied := EnsureReactImport(buf, "useState")
	if !applied {
		t.Errorf("expected EnsureReactImport to apply for new import")
	}

	text := buf.Text()
	if !strings.Contains(text, "import { useState } from 'react';") {
		t.Errorf("expected text to contain 'import { useState } from 'react';', got:\n%s", text)
	}

	// Should not re-import if already present
	secondApplied := EnsureReactImport(buf, "useState")
	if secondApplied {
		t.Errorf("expected second EnsureReactImport to return false when already imported")
	}
}

func TestEnsureReactImportMergeWithDefault(t *testing.T) {
	buf := NewBuffer("import React from 'react';\n\nexport const MyComp = () => <div>Hello</div>;\n")

	applied := EnsureReactImport(buf, "useEffect")
	if !applied {
		t.Errorf("expected EnsureReactImport to apply")
	}

	text := buf.Text()
	if !strings.Contains(text, "import React, { useEffect } from 'react';") {
		t.Errorf("expected merged import, got:\n%s", text)
	}
}

func TestEnsureReactImportMergeWithNamed(t *testing.T) {
	buf := NewBuffer("import { useState } from 'react';\n\nexport const MyComp = () => <div>Hello</div>;\n")

	applied := EnsureReactImport(buf, "useCallback")
	if !applied {
		t.Errorf("expected EnsureReactImport to apply")
	}

	text := buf.Text()
	if !strings.Contains(text, "import { useState, useCallback } from 'react';") {
		t.Errorf("expected merged named import, got:\n%s", text)
	}
}

func TestEnsureWorkspaceImport(t *testing.T) {
	buf := NewBuffer("export const App = () => <div>App</div>;\n")

	currentFile := "/workspace/src/App.jsx"
	sourceFile := "/workspace/src/components/Button.jsx"

	applied := EnsureWorkspaceImport(buf, "Button", sourceFile, currentFile)
	if !applied {
		t.Errorf("expected EnsureWorkspaceImport to apply")
	}

	text := buf.Text()
	if !strings.Contains(text, "import { Button } from './components/Button';") {
		t.Errorf("expected relative import for Button, got:\n%s", text)
	}

	// Should not re-import if already imported
	secondApplied := EnsureWorkspaceImport(buf, "Button", sourceFile, currentFile)
	if secondApplied {
		t.Errorf("expected second EnsureWorkspaceImport to return false")
	}
}

func TestIsSymbolImportedMultiLineAndBarrel(t *testing.T) {
	// User case: Multi-line barrel import from '@/components/index'
	source := `import {
  Button,
  Checkbox,
  FileDropzone,
  Label,
  SelectObject,
  TextareaInput
} from '@/components/index';

export const MyPage = () => {
  return <Button>Click</Button>;
};
`
	buf := NewBuffer(source)

	if !IsSymbolImported(buf, "Button") {
		t.Errorf("Expected Button to be recognized as already imported")
	}
	if !IsSymbolImported(buf, "SelectObject") {
		t.Errorf("Expected SelectObject to be recognized as already imported")
	}
	if IsSymbolImported(buf, "NonExistentComponent") {
		t.Errorf("NonExistentComponent should not be reported as imported")
	}

	// Calling EnsureTextEditImport for Button must return false and NOT duplicate the import line
	applied := EnsureTextEditImport(buf, "Button", "import { Button } from '../../components/ui/button';\n")
	if applied {
		t.Errorf("Expected EnsureTextEditImport to reject duplicate import for Button")
	}

	// Verify text has not been modified
	if strings.Contains(buf.Text(), "../../components/ui/button") {
		t.Errorf("Buffer should not contain duplicate import path")
	}
}

func TestEnsureTextEditImportDuplicatePrevention(t *testing.T) {
	buf := NewBuffer("export const Page = () => <div>Page</div>;\n")

	// 1st call: Should apply
	applied1 := EnsureTextEditImport(buf, "Button", "import { Button } from '../../components/ui/button';\n")
	if !applied1 {
		t.Errorf("1st call to EnsureTextEditImport should apply")
	}

	// 2nd call: Must reject (symbol already imported)
	applied2 := EnsureTextEditImport(buf, "Button", "import { Button } from '../../components/ui/button';\n")
	if applied2 {
		t.Errorf("2nd call to EnsureTextEditImport must be rejected")
	}

	// 3rd call with different path: Must still reject because Button is already imported!
	applied3 := EnsureTextEditImport(buf, "Button", "import { Button } from '@/components/index';\n")
	if applied3 {
		t.Errorf("3rd call with different path must be rejected because Button is already in file")
	}

	// Verify count of "Button" in import lines is exactly 1
	count := strings.Count(buf.Text(), "import { Button }")
	if count != 1 {
		t.Errorf("Expected exactly 1 import of Button, got %d. Content:\n%s", count, buf.Text())
	}
}
