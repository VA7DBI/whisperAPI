# whisperAPI model download helpers

MODELS_DIR ?= ./models
PARAKEET_MODEL_REPO ?= https://huggingface.co/istupakov/parakeet-tdt-0.6b-v3-onnx
SILERO_VAD_VERSION ?= v6.2.1
SILERO_VAD_SHA256 ?= 1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3
SILERO_VAD_URL := https://github.com/snakers4/silero-vad/raw/$(SILERO_VAD_VERSION)/src/silero_vad/data/silero_vad.onnx

.PHONY: help models models-int8 models-fp32 models-silero-vad

help: ## Show available make targets
	@echo "whisperAPI Make targets"
	@echo ""
	@echo "  make models            Download Parakeet int8 models (default)"
	@echo "  make models-int8       Download Parakeet int8 models (~670MB)"
	@echo "  make models-fp32       Download Parakeet fp32 models (~2.5GB)"
	@echo "  make models-silero-vad Download and verify Silero VAD model"

models: models-int8 ## Download models (default: int8)

models-silero-vad: ## Download and verify the Silero VAD model
	@mkdir -p $(MODELS_DIR)
	@echo "Downloading Silero VAD $(SILERO_VAD_VERSION)..."
	@curl -L -o $(MODELS_DIR)/silero_vad.onnx "$(SILERO_VAD_URL)"
	@echo "$(SILERO_VAD_SHA256)  $(MODELS_DIR)/silero_vad.onnx" | sha256sum -c -
	@echo "Silero VAD model verified"

models-int8: models-silero-vad ## Download Parakeet int8 models
	@mkdir -p $(MODELS_DIR)
	@echo "Downloading Parakeet int8 models from $(PARAKEET_MODEL_REPO)..."
	@curl -L -o $(MODELS_DIR)/config.json "$(PARAKEET_MODEL_REPO)/resolve/main/config.json"
	@curl -L -o $(MODELS_DIR)/vocab.txt "$(PARAKEET_MODEL_REPO)/resolve/main/vocab.txt"
	@curl -L -o $(MODELS_DIR)/nemo128.onnx "$(PARAKEET_MODEL_REPO)/resolve/main/nemo128.onnx"
	@curl -L -o $(MODELS_DIR)/encoder-model.int8.onnx "$(PARAKEET_MODEL_REPO)/resolve/main/encoder-model.int8.onnx"
	@curl -L -o $(MODELS_DIR)/decoder_joint-model.int8.onnx "$(PARAKEET_MODEL_REPO)/resolve/main/decoder_joint-model.int8.onnx"
	@echo "Parakeet int8 models downloaded to $(MODELS_DIR)"

models-fp32: models-silero-vad ## Download Parakeet fp32 models
	@mkdir -p $(MODELS_DIR)
	@echo "Downloading Parakeet fp32 models from $(PARAKEET_MODEL_REPO)..."
	@curl -L -o $(MODELS_DIR)/config.json "$(PARAKEET_MODEL_REPO)/resolve/main/config.json"
	@curl -L -o $(MODELS_DIR)/vocab.txt "$(PARAKEET_MODEL_REPO)/resolve/main/vocab.txt"
	@curl -L -o $(MODELS_DIR)/nemo128.onnx "$(PARAKEET_MODEL_REPO)/resolve/main/nemo128.onnx"
	@curl -L -o $(MODELS_DIR)/encoder-model.onnx "$(PARAKEET_MODEL_REPO)/resolve/main/encoder-model.onnx"
	@curl -L -o $(MODELS_DIR)/encoder-model.onnx.data "$(PARAKEET_MODEL_REPO)/resolve/main/encoder-model.onnx.data"
	@curl -L -o $(MODELS_DIR)/decoder_joint-model.onnx "$(PARAKEET_MODEL_REPO)/resolve/main/decoder_joint-model.onnx"
	@echo "Parakeet fp32 models downloaded to $(MODELS_DIR)"
