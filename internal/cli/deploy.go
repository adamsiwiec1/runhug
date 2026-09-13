package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/family"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/jobs"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/runpod"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/sizing"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/store"
)

func cmdDeploy(args []string) error {
	fs := newFlagSet("deploy")
	yes := fs.Bool("yes", false, "create without a prompt")
	dry := fs.Bool("dry-run", false, "print the plan only")
	gpu := fs.String("gpu", "", "serverless GPU pool (ADA_24, AMPERE_80, …)")
	gpuCount := fs.Int("gpu-count", 0, "GPUs per worker (default: sized)")
	maxLen := fs.Int("max-len", 8192, "MAX_MODEL_LEN")
	minWorkers := fs.Int("min-workers", 0, "minimum workers (nonzero bills around the clock)")
	maxWorkers := fs.Int("max-workers", 3, "maximum workers")
	idle := fs.Int("idle-timeout", 5, "seconds before an idle worker scales down")
	flashboot := fs.String("flashboot", "FLASHBOOT", "OFF, FLASHBOOT, or PRIORITY_FLASHBOOT")
	image := fs.String("image", runpod.DefaultImage, "worker image")
	disk := fs.Int("disk", 0, "container disk GB (0 = sized from the repo)")
	name := fs.String("name", "", "endpoint name")
	quant := fs.String("quant", "", "QUANTIZATION override (awq, gptq, …)")
	noFamily := fs.Bool("no-family", false, "skip Qwen/Mistral/Llama env defaults")
	trust := fs.Bool("trust-remote-code", false, "set TRUST_REMOTE_CODE=true")
	force := fs.Bool("force", false, "deploy gated models without HF_TOKEN, or non-vLLM formats")
	smoke := fs.Bool("smoke", false, "run a short completion after create (bills a cold start)")
	asJSON := fs.Bool("json", false, "print JSON")
	var extra stringsFlag
	fs.Var(&extra, "env", "extra KEY=VALUE (repeatable)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: runpod-vllm-proxy deploy <org/model>")
	}
	modelID := fs.Arg(0)
	env := config.Load()
	if err := env.RequireRunpod(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	model, err := hf.New(env.HFToken).Get(ctx, modelID)
	if err != nil {
		return err
	}
	modelID = model.RepoID()
	format := hf.DetectFormat(*model)
	if format.Engine == hf.EngineGGUF && !*force {
		return fmt.Errorf("%s is GGUF; vLLM will not load it. Use a llama.cpp worker, or pass --force", modelID)
	}
	if model.IsGated() && env.HFToken == "" && !*force {
		return fmt.Errorf("%s is gated; set HF_TOKEN or pass --force (the worker will 403 on download)", modelID)
	}

	est := inspectEstimate(*model, format, *maxLen)
	rp := runpod.New(env.RunpodAPIKey)
	gpus, err := rp.ListGPUs(ctx)
	if err != nil {
		return err
	}
	choice, err := runpod.Pick(gpus, est.RequiredGB, *gpu, *gpuCount)
	if err != nil {
		return err
	}

	diskGB := *disk
	if diskGB <= 0 {
		diskGB = est.DiskGB
	}
	endpointName := *name
	if endpointName == "" {
		endpointName = slug(modelID)
	}

	workerEnv := map[string]string{
		"MODEL_NAME":                        modelID,
		"RAW_OPENAI_OUTPUT":                 "1",
		"GPU_MEMORY_UTILIZATION":            "0.90",
		"OPENAI_SERVED_MODEL_NAME_OVERRIDE": modelID,
		"MAX_MODEL_LEN":                     fmt.Sprintf("%d", *maxLen),
	}
	if env.HFToken != "" {
		workerEnv["HF_TOKEN"] = env.HFToken
	}
	if !*noFamily {
		for k, v := range family.EnvFor(modelID) {
			workerEnv[k] = v
		}
	}
	q := *quant
	if q == "" {
		q = format.Quant
	}
	if q != "" {
		workerEnv["QUANTIZATION"] = q
	}
	if *trust {
		workerEnv["TRUST_REMOTE_CODE"] = "true"
	}
	if choice.GPUCount > 1 {
		workerEnv["TENSOR_PARALLEL_SIZE"] = fmt.Sprintf("%d", choice.GPUCount)
	}
	userEnv, err := parseKV(extra)
	if err != nil {
		return err
	}
	for k, v := range userEnv {
		workerEnv[k] = v
	}

	req := runpod.CreateEndpointRequest{
		Name:  endpointName,
		Image: *image,
		Disk:  diskGB,
		Env:   workerEnv,
		GPU: runpod.GPUConfig{
			Pools: []string{choice.Pool.ID},
			Count: choice.GPUCount,
		},
		Workers:   &runpod.Workers{Min: *minWorkers, Max: *maxWorkers},
		Scaling:   &runpod.Scaling{Type: "QUEUE_DELAY", Value: 4, IdleTimeout: *idle},
		Timeout:   600000,
		Flashboot: strings.ToUpper(*flashboot),
	}

	printPlan(modelID, format, est, choice, req, env.HFToken != "")

	if *dry {
		if *asJSON {
			return writeJSON(map[string]any{"plan": req, "choice": choice, "estimate": est})
		}
		fmt.Println(dim("dry-run: nothing created"))
		return nil
	}
	if !*yes {
		if !confirm(fmt.Sprintf("Create this endpoint? Serverless GPU ~$%.2f/hr while a worker is up", choice.HourlyUSD)) {
			return fmt.Errorf("aborted")
		}
	}

	created, err := rp.CreateEndpoint(ctx, req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "409") || strings.Contains(strings.ToLower(err.Error()), "collision") {
			req.Name = endpointName + "-" + time.Now().UTC().Format("150405")
			created, err = rp.CreateEndpoint(ctx, req)
		}
		if err != nil {
			return err
		}
	}

	reg, _, err := store.Load()
	if err != nil {
		return err
	}
	reg.Put(store.Model{
		HFRepo:     modelID,
		EndpointID: created.ID,
		GPUPool:    choice.Pool.ID,
		GPUCount:   choice.GPUCount,
		Image:      req.Image,
		HourlyUSD:  choice.HourlyUSD,
		CreatedAt:  time.Now().UTC(),
	})
	reg.Current = modelID
	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "%s saved endpoint %s but registry write failed: %v\n", yellow("warning:"), created.ID, err)
	}

	openai := runpod.OpenAIURL(created.ID)
	result := map[string]any{
		"endpoint_id": created.ID,
		"name":        created.Name,
		"model":       modelID,
		"gpu_pool":    choice.Pool.ID,
		"gpu_count":   choice.GPUCount,
		"hourly_usd":  choice.HourlyUSD,
		"openai_url":  openai,
	}

	if *smoke {
		fmt.Fprintln(os.Stderr, "smoke: short completion (first job is a multi-minute cold start)…")
		text, err := jobs.Chat(env.RunpodAPIKey, created.ID, "Say hello in one short sentence.", 32)
		if err != nil {
			result["smoke_error"] = err.Error()
			fmt.Fprintf(os.Stderr, "smoke failed: %v\n", err)
		} else {
			result["smoke"] = text
			fmt.Fprintf(os.Stderr, "smoke: %s\n", text)
		}
	}

	if *asJSON {
		return writeJSON(result)
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%s  %s  (%s)\n", green("Created"), cyan(created.ID), created.Name)
	printKV(os.Stdout, "model", bold(modelID))
	printKV(os.Stdout, "hub", cyan(hubLink(modelID)))
	printKV(os.Stdout, "openai", cyan(openai))
	printKV(os.Stdout, "billing", fmt.Sprintf("scale-to-zero (min workers %d). A running worker is ~$%.2f/hr on %s.", *minWorkers, choice.HourlyUSD, choice.Pool.ID))
	fmt.Fprintln(os.Stdout)
	commands(os.Stdout, "Next:",
		"runpod-vllm-proxy proxy",
		"curl http://127.0.0.1:8080/v1/models",
		"runpod-vllm-proxy delete "+modelID,
	)
	return nil
}

func inspectEstimate(m hf.Model, format hf.Format, maxLen int) sizing.Estimate {
	return sizing.EstimateModel(m, format, maxLen)
}

func inspectGPU(ctx context.Context, apiKey string, required float64, pool string) (string, any, error) {
	gpus, err := runpod.New(apiKey).ListGPUs(ctx)
	if err != nil {
		return "", nil, err
	}
	c, err := runpod.Pick(gpus, required, pool, 0)
	if err != nil {
		return "", nil, err
	}
	text := fmt.Sprintf("%s (%s, %.0f GB)  $%.2f/hr serverless  stock %s  ×%d GPU",
		c.Pool.ID, c.Pool.ExampleGPU, c.Pool.MemoryGB, c.HourlyUSD, c.Pool.Availability, c.GPUCount)
	if c.Next != nil {
		text += fmt.Sprintf("  | next %s $%.2f/hr", c.Next.ID, c.Next.PricePerHour)
	}
	return text, c, nil
}

func printPlan(modelID string, format hf.Format, est sizing.Estimate, c runpod.Choice, req runpod.CreateEndpointRequest, hasHF bool) {
	heading(os.Stdout, "Plan")
	printKV(os.Stdout, "model", bold(modelID))
	fmt.Fprintf(os.Stdout, "  %s  %s", dim(padRight("format", 9)), format.Engine)
	if format.Quant != "" {
		fmt.Fprintf(os.Stdout, " (%s)", format.Quant)
	}
	fmt.Println()
	printKV(os.Stdout, "params", fmt.Sprintf("%s (%s)", est.ParamsLabel(), dash(est.ParamsSource)))
	if est.WeightGB > 0 {
		printKV(os.Stdout, "vram", fmt.Sprintf("%.1f GB weights → %.1f GB with overhead", est.WeightGB, est.RequiredGB))
	}
	printKV(os.Stdout, "gpu", fmt.Sprintf("%s ×%d  %s  $%.2f/hr  stock %s",
		c.Pool.ID, c.GPUCount, c.Pool.ExampleGPU, c.HourlyUSD, c.Pool.Availability))
	printKV(os.Stdout, "why", c.Reason)
	printKV(os.Stdout, "image", req.Image)
	printKV(os.Stdout, "disk", fmt.Sprintf("%d GB", req.Disk))
	printKV(os.Stdout, "workers", fmt.Sprintf("min=%d max=%d flashboot=%s", req.Workers.Min, req.Workers.Max, req.Flashboot))
	keys := make([]string, 0, len(req.Env))
	for k := range req.Env {
		if k == "HF_TOKEN" {
			continue
		}
		keys = append(keys, k+"="+req.Env[k])
	}
	printKV(os.Stdout, "env", strings.Join(keys, "  "))
	if hasHF {
		printKV(os.Stdout, "hf_token", "set (not printed)")
	}
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

func slug(modelID string) string {
	s := strings.ToLower(modelID)
	s = strings.ReplaceAll(s, "/", "-")
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "vllm-model"
	}
	s = "vllm-" + s
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}
