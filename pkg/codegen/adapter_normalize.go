package codegen

func normalizedAdapterPlan(plan *Plan) *Plan {
	if plan == nil || plan.AdapterPlan == nil {
		return plan
	}
	clone := *plan
	clone.BoundaryParams = normalizedAdapterParams(plan)
	clone.ReconstructedParams = normalizedAdapterReconstructedParams(plan)
	clone.Results = normalizedAdapterResults(plan)
	clone.ResultDTO = BuildResultDTO(plan.CutPoint.FuncName, clone.Results)
	if clone.ResultDTO != nil {
		clone.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: clone.ResultDTO.Name}
	} else {
		clone.ReturnCodec = ReturnCodecFor(clone.Results)
	}
	return &clone
}

func normalizedAdapterParams(plan *Plan) []Param {
	params := append([]Param(nil), plan.BoundaryParams...)
	for _, rp := range plan.ReconstructedParams {
		if adapterInputTransformForParam(plan, rp.Param.Name) != nil {
			params = append(params, rp.Param)
		}
	}
	for _, transform := range plan.AdapterPlan.InputTransforms {
		for i := range params {
			if params[i].Name != transform.ParamName {
				continue
			}
			params[i].Name = adapterInputName(transform, params[i])
			params[i].JSONName = toSnake(params[i].Name)
			params[i].GoType = transform.ToType
			params[i].QualifiedGoType = transform.ToType
			params[i].TypePackagePath = ""
			params[i].TypePackageAlias = ""
			params[i].Codec = CodecJSON
		}
	}
	return params
}

func normalizedAdapterReconstructedParams(plan *Plan) []ReconstructedParam {
	reconstructed := make([]ReconstructedParam, 0, len(plan.ReconstructedParams))
	for _, param := range plan.ReconstructedParams {
		if adapterInputTransformForParam(plan, param.Param.Name) != nil {
			continue
		}
		reconstructed = append(reconstructed, param)
	}
	return reconstructed
}

func adapterInputTransformForParam(plan *Plan, name string) *AdapterPattern {
	if plan == nil || plan.AdapterPlan == nil {
		return nil
	}
	for i := range plan.AdapterPlan.InputTransforms {
		if plan.AdapterPlan.InputTransforms[i].ParamName == name {
			return &plan.AdapterPlan.InputTransforms[i]
		}
	}
	return nil
}

func normalizedAdapterResults(plan *Plan) []Result {
	results := append([]Result(nil), plan.Results...)
	outputBySlot := map[int]AdapterPattern{}
	for _, transform := range plan.AdapterPlan.OutputTransforms {
		slot := firstResultSlotByType(results, transform.FromType)
		if slot >= 0 {
			outputBySlot[slot] = transform
		}
	}
	for i := range results {
		transform, ok := outputBySlot[results[i].Index]
		if !ok {
			continue
		}
		results[i].Name = adapterOutputName(results[i])
		results[i].JSONName = toSnake(results[i].Name)
		results[i].GoType = transform.ToType
		results[i].QualifiedGoType = transform.ToType
		results[i].TypePackagePath = ""
		results[i].TypePackageAlias = ""
		results[i].Codec = CodecJSON
	}
	return results
}

func adapterInputName(transform AdapterPattern, param Param) string {
	if transform.Name == "multipart_file_read_all" {
		return "input"
	}
	if param.Name == "" {
		return "input"
	}
	return param.Name + "Value"
}

func adapterOutputName(result Result) string {
	if result.Name == "" || result.Name == "result" {
		return "result0"
	}
	return result.Name
}

func firstResultSlotByType(results []Result, typ string) int {
	for _, result := range results {
		if result.GoType == typ {
			return result.Index
		}
	}
	return -1
}
