#!/usr/bin/env python3
"""
scripts/validate_openapi.py
OpenAPI 3.0.3 Contract & Schema Validator.
Validates all system OpenAPI specifications:
  - docs/openapi.yaml
  - backend/api-gateway/swagger/openapi.yaml
  - backend/api-gateway/swagger/openapi.json
  - backend/catalog-service/openapi.json
  - backend/inventory-service/openapi.json
  - backend/booking-service/openapi.json
  - backend/auth-service/openapi.json
  - backend/notification-service/openapi.json

Zero external dependencies: uses Python 3 standard library exclusively,
with optional PyYAML support if installed.
"""

import json
import os
import re
import sys
from typing import Any, Dict, List, Tuple

# Try importing pyyaml if available
try:
    import yaml
    HAS_PYYAML = True
except ImportError:
    yaml = None
    HAS_PYYAML = False


def parse_simple_yaml(text: str) -> Dict[str, Any]:
    """
    Lightweight fallback YAML parser for OpenAPI 3.0 documents
    when PyYAML is not installed.
    """
    data: Dict[str, Any] = {"paths": {}, "info": {}}
    current_section = None
    current_path = None
    current_method = None

    for line in text.splitlines():
        line_stripped = line.strip()
        if not line_stripped or line_stripped.startswith("#"):
            continue

        # Match top-level keys
        if not line.startswith(" ") and ":" in line:
            key = line.split(":", 1)[0].strip()
            val = line.split(":", 1)[1].strip()
            current_section = key
            if val:
                data[key] = val
            elif key not in data:
                data[key] = {}
            current_path = None
            current_method = None
            continue

        indent = len(line) - len(line.lstrip(" "))

        if current_section == "info":
            if ":" in line_stripped:
                k, v = line_stripped.split(":", 1)
                data["info"][k.strip()] = v.strip().strip('"').strip("'")

        elif current_section == "paths":
            if indent == 2 and line_stripped.startswith("/") and ":" in line_stripped:
                current_path = line_stripped.split(":", 1)[0].strip()
                if current_path not in data["paths"]:
                    data["paths"][current_path] = {}
                current_method = None
            elif indent == 4 and current_path and ":" in line_stripped:
                m = line_stripped.split(":", 1)[0].strip().lower()
                if m in ("get", "post", "put", "delete", "patch", "options", "head"):
                    current_method = m
                    data["paths"][current_path][current_method] = {"responses": {}}
            elif indent >= 6 and current_path and current_method:
                if "responses:" in line_stripped:
                    pass
                elif line_stripped.startswith("'") or line_stripped.startswith('"') or line_stripped.isdigit():
                    resp_code = line_stripped.split(":", 1)[0].strip().strip("'").strip('"')
                    data["paths"][current_path][current_method]["responses"][resp_code] = {}

    return data


def load_spec(filepath: str) -> Dict[str, Any]:
    if not os.path.exists(filepath):
        raise FileNotFoundError(f"Specification file not found: {filepath}")

    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()

    if filepath.endswith(".json"):
        return json.loads(content)

    if filepath.endswith((".yaml", ".yml")):
        if HAS_PYYAML:
            return yaml.safe_load(content)
        return parse_simple_yaml(content)

    raise ValueError(f"Unsupported file format: {filepath}")


def validate_spec(name: str, spec: Dict[str, Any]) -> List[str]:
    errors = []

    # 1. OpenAPI version check
    openapi_version = str(spec.get("openapi", ""))
    if not openapi_version.startswith("3.0"):
        errors.append(f"Invalid openapi version '{openapi_version}', expected 3.0.x")

    # 2. Info object check
    info = spec.get("info")
    if not isinstance(info, dict):
        errors.append("Missing required 'info' object")
    else:
        if not info.get("title"):
            errors.append("info object missing required 'title'")
        if not info.get("version"):
            errors.append("info object missing required 'version'")

    # 3. Paths object check
    paths = spec.get("paths")
    if not isinstance(paths, dict):
        errors.append("Missing required 'paths' object")
    elif len(paths) == 0:
        errors.append("'paths' object cannot be empty")
    else:
        valid_methods = {"get", "post", "put", "delete", "patch", "options", "head"}
        for path_key, path_item in paths.items():
            if not isinstance(path_item, dict):
                continue
            for method, operation in path_item.items():
                if method.lower() not in valid_methods:
                    continue
                if not isinstance(operation, dict):
                    errors.append(f"Operation {method.upper()} {path_key} must be an object")
                    continue
                responses = operation.get("responses")
                if not isinstance(responses, dict) or len(responses) == 0:
                    errors.append(f"Operation {method.upper()} {path_key} missing responses object")

    return errors


def main() -> int:
    base_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

    targets = [
        ("Unified Docs (YAML)", os.path.join(base_dir, "docs/openapi.yaml")),
        ("Gateway Swagger (YAML)", os.path.join(base_dir, "backend/api-gateway/swagger/openapi.yaml")),
        ("Gateway Swagger (JSON)", os.path.join(base_dir, "backend/api-gateway/swagger/openapi.json")),
        ("Catalog Service (JSON)", os.path.join(base_dir, "backend/catalog-service/openapi.json")),
        ("Inventory Service (JSON)", os.path.join(base_dir, "backend/inventory-service/openapi.json")),
        ("Booking Service (JSON)", os.path.join(base_dir, "backend/booking-service/openapi.json")),
        ("Auth Service (JSON)", os.path.join(base_dir, "backend/auth-service/openapi.json")),
        ("Notification Service (JSON)", os.path.join(base_dir, "backend/notification-service/openapi.json")),
    ]

    print("======================================================================")
    print("🔍 OpenAPI 3.0.3 Contract & Schema Validation Suite")
    print("======================================================================")

    total_specs = len(targets)
    passed_specs = 0
    failed_specs = 0

    for name, path in targets:
        rel_path = os.path.relpath(path, base_dir)
        try:
            spec = load_spec(path)
            errors = validate_spec(name, spec)
            if errors:
                failed_specs += 1
                print(f" ❌ FAIL: {name} ({rel_path})")
                for err in errors:
                    print(f"    - {err}")
            else:
                passed_specs += 1
                path_count = len(spec.get("paths", {}))
                print(f" ✅ PASS: {name} ({rel_path}) — OpenAPI {spec.get('openapi')} ({path_count} paths)")
        except Exception as e:
            failed_specs += 1
            print(f" ❌ ERROR: {name} ({rel_path}): {e}")

    print("======================================================================")
    print(f"Summary: {passed_specs}/{total_specs} specifications validated successfully.")
    print("======================================================================")

    if failed_specs > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
