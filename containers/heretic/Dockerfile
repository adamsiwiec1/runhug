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

# heretic CLI (git master: stable PyPI releases are interactive-only and
# lack the headless --model-action/--upload-repo-id flags) + dashboard runtime.
RUN pip install -U --no-cache-dir \
        "git+https://github.com/p-e-w/heretic.git" \
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