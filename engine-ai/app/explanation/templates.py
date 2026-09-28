"""Deterministic Spanish texts per anomaly type, built only from the facts."""

from dataclasses import dataclass

from app.analysis.models import DATA_QUALITY, EXPLAINABLE_ANOMALY, FALSE_POSITIVE, REAL_ANOMALY
from app.explanation.facts import Facts

# The action category is decided by code, never by the LLM.
ACTION_BASE = {
    REAL_ANOMALY: "Investigar medidor e instalación",
    DATA_QUALITY: "Validar medidor / lecturas",
    EXPLAINABLE_ANOMALY: "Validar operación",
    FALSE_POSITIVE: "No escalar",
}


@dataclass(frozen=True)
class Texts:
    reason: str
    explanation: str
    action_detail: str


def recommended_action(anomaly_type: str, detail: str) -> str:
    """Fixed base action for the type, then the specific detail."""
    return f"{ACTION_BASE[anomaly_type]}. {detail}"


def _sentences(*parts: str) -> str:
    return " ".join(part for part in parts if part)


def _consumption(f: Facts) -> str:
    return f"El consumo diario pasó de un baseline de {f.baseline} a {f.current} ({f.variation})."


def _onset(f: Facts) -> str:
    if not f.change_start:
        return ""
    return f"El cambio empieza el {f.change_start} y persiste hasta el final de los datos."


def _electrical(f: Facts, lead: str) -> str:
    if not f.changed_vars:
        return ""
    changes = ", ".join(f"{label} {change}" for label, change in f.changed_vars)
    return f"{lead}: {changes}."


def _transient(f: Facts) -> str:
    if not f.transient:
        return ""
    t = f.transient
    return (
        f"El consumo se desvió {t.deviation} de media entre el {t.start} y el {t.end} ({t.hours})."
    )


def _recovery(f: Facts) -> str:
    return (
        f"Después se recuperó: el último día marca {f.current} frente a un baseline "
        f"de {f.baseline} ({f.variation})."
    )


def _checks(f: Facts) -> str:
    if not f.failed_checks:
        return ""
    checks = ", ".join(f"{label} ({hours})" for label, hours in f.failed_checks)
    return f"Fallan chequeos de calidad de datos: {checks}."


def _events(f: Facts, lead: str) -> str:
    if not f.events:
        return ""
    items = ", ".join(f"{e.label} ({e.moment})" for e in f.events)
    return f"{lead}: {items}."


def _unexplained_events(f: Facts, finding: str) -> str:
    if not f.events:
        return ""
    if len(f.events) == 1:
        return f"El único evento registrado es '{f.events[0].label}', que no explica {finding}."
    labels = ", ".join(f"'{e.label}'" for e in f.events)
    return f"Los eventos registrados ({labels}) no explican {finding}."


def _real(f: Facts) -> Texts:
    if f.transient and not f.change_start:
        t = f.transient
        return Texts(
            reason=f"Desviación transitoria de {t.hours} ({t.deviation}) sin evento operativo "
            "que la explique.",
            explanation=_sentences(
                _transient(f), _recovery(f), _checks(f), _unexplained_events(f, "la desviación")
            ),
            action_detail=f"Revisar qué ocurrió en la instalación entre el {t.start} y el {t.end}.",
        )
    since = f" desde el {f.change_start}" if f.change_start else ""
    direction = "bajo" if f.variation.startswith("-") else "sobre"
    detail = f"Revisar la carga conectada{since}"
    if f.power_factor_drop:
        detail += " y la caída del factor de potencia"
    return Texts(
        reason=f"Consumo {f.variation} {direction} el baseline{since}, sin evento operativo "
        "que lo explique.",
        explanation=_sentences(
            _consumption(f),
            _onset(f),
            _electrical(f, "Cambiaron también las variables eléctricas"),
            _checks(f),
            _unexplained_events(f, "el cambio"),
        ),
        action_detail=f"{detail}.",
    )


def _data_quality(f: Facts) -> Texts:
    return Texts(
        reason=f"Consumo estable ({f.variation}) con lecturas eléctricas inconsistentes.",
        explanation=_sentences(
            f"El consumo diario se mantiene en {f.current} frente a un baseline de "
            f"{f.baseline} ({f.variation}), pero las lecturas eléctricas no son coherentes.",
            _checks(f),
            _events(f, "Evento registrado"),
        ),
        action_detail="Revisar el medidor y su comunicación antes de usar estos datos.",
    )


def _explainable(f: Facts) -> Texts:
    since = f" desde el {f.change_start}" if f.change_start else ""
    return Texts(
        reason=f"Consumo {f.variation}{since}, coincide con un cambio operativo registrado.",
        explanation=_sentences(
            _consumption(f),
            _onset(f),
            _electrical(f, "Las variables eléctricas acompañan el cambio"),
            _events(f, "Coincide con el evento registrado"),
        ),
        action_detail="Confirmar con operación que la nueva carga es la esperada y actualizar "
        "el baseline.",
    )


def _false_positive(f: Facts) -> Texts:
    if f.transient:
        kind = "Caída" if f.transient.deviation.startswith("-") else "Subida"
        reason = (
            f"{kind} transitoria de {f.transient.hours} ({f.transient.deviation}) dentro de "
            "una parada programada."
        )
    else:
        reason = "Desviación transitoria dentro de una parada programada."
    return Texts(
        reason=reason,
        explanation=_sentences(
            _transient(f),
            _recovery(f),
            _events(f, "La desviación queda dentro de la ventana del evento registrado"),
        ),
        action_detail="Registrar como explicado por la parada programada.",
    )


TEMPLATES = {
    REAL_ANOMALY: _real,
    DATA_QUALITY: _data_quality,
    EXPLAINABLE_ANOMALY: _explainable,
    FALSE_POSITIVE: _false_positive,
}


def template_texts(facts: Facts) -> Texts:
    return TEMPLATES[facts.type](facts)
