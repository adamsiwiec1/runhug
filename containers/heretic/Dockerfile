# runhug heretic training pod
#
# Runs the heretic (abliteration) optimizer headlessly on a RunPod GPU pod,
# tailing its console output so a small FastAPI dashboard can render live
# progress (refusals vs KL per trial) and the raw log.
FROM runpod/pytorch:2.4.0-py3.11-cuda12.4.1-devel-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    TOKENIZERS_PARALLELISM=false \
    HF_HUB_DISABLE_SYMLINKS_WARNING=1

# transformers 5.x (forced by heretic master) requires torch >= 2.5; the
# runpod base ships torch 2.4.1, which makes transformers disable torch at
# import (NameError: nn). Upgrade the whole torch stack: the base's
# torchvision 0.19.1 / torchaudio 2.4.1 both pin torch==2.4.1, so leaving
# them forces pip to downgrade torch to 2.4.1 on the next install.
RUN pip install -U --no-cache-dir \
        --index-url https://download.pytorch.org/whl/cu124 \
        torch==2.5.1 torchvision==0.20.1 torchaudio==2.5.1 \
 && python -c "import torch; assert torch.__version__ >= '2.5.0', f'expected torch>=2.5, found {torch.__version__}'; print('torch', torch.__version__)"

# heretic CLI (git master: stable PyPI releases are interactive-only and
# lack the headless --model-action/--upload-repo-id flags) + dashboard runtime.
# accelerate/peft are co-upgraded so transformers 5.x imports cleanly.
RUN pip install -U --no-cache-dir \
        "git+https://github.com/p-e-w/heretic.git" \
        accelerate \
        peft \
        fastapi \
        "uvicorn[standard]" \
        jinja2

WORKDIR /workspace
COPY run_pipeline.py dashboard.py /workspace/
COPY templates/ /workspace/templates/

RUN mkdir -p /output /workspace/runhug && \
    chmod -R a+rw /output /workspace/runhug

EXPOSE 8080
ENTRYPOINT ["python", "/workspace/run_pipeline.py"]