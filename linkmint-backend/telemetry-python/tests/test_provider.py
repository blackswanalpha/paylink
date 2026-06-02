from linkmint_telemetry.provider import _grpc_target, _ratio, _truthy, init_telemetry


def test_truthy():
    assert _truthy("YES") and _truthy("1") and _truthy("on")
    assert not _truthy(None) and not _truthy("no") and not _truthy("")


def test_grpc_target():
    assert _grpc_target("http://tempo:4317/") == ("tempo:4317", True)
    assert _grpc_target("https://tempo:4317") == ("tempo:4317", False)
    assert _grpc_target("tempo:4317") == ("tempo:4317", True)


def test_ratio(monkeypatch):
    monkeypatch.setenv("OTEL_TRACES_SAMPLER_ARG", "0.5")
    assert _ratio() == 0.5
    monkeypatch.setenv("OTEL_TRACES_SAMPLER_ARG", "bad")
    assert _ratio() == 1.0
    monkeypatch.setenv("OTEL_TRACES_SAMPLER_ARG", "9")
    assert _ratio() == 1.0
    monkeypatch.delenv("OTEL_TRACES_SAMPLER_ARG", raising=False)
    assert _ratio() == 1.0


def test_init_disabled(monkeypatch):
    monkeypatch.setenv("OTEL_SDK_DISABLED", "true")
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://x:4317")
    assert init_telemetry("svc")() is None


def test_init_no_endpoint(monkeypatch):
    monkeypatch.delenv("OTEL_SDK_DISABLED", raising=False)
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
    init_telemetry("svc")()


def test_init_with_endpoint(monkeypatch):
    monkeypatch.delenv("OTEL_SDK_DISABLED", raising=False)
    monkeypatch.setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:14317")
    monkeypatch.setenv("OTEL_SERVICE_NAME", "override-svc")
    shutdown = init_telemetry("svc", "v1")
    shutdown()
