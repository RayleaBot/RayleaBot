from .platforms import (
    normalize_platform,
    platform_ids,
    platform_service_aliases,
    platform_service_names,
)


PLATFORM_SERVICE_NAMES = {platform: platform_service_names(platform) for platform in platform_ids()}

SERVICE_NAMES = PLATFORM_SERVICE_NAMES["bilibili"]

PLATFORM_SERVICE_ALIASES = {platform: platform_service_aliases(platform) for platform in platform_ids()}


def service_names_for(platform):
    return PLATFORM_SERVICE_NAMES.get(normalize_platform(platform), SERVICE_NAMES)


def service_order_for(platform):
    return list(service_names_for(platform).keys())


def service_types_for(platform):
    return [service for service in service_order_for(platform) if service != "all"]


def service_enabled(subscription, service):
    platform = normalize_platform(subscription.get("platform"))
    services = set(normalize_services(subscription.get("services"), platform))
    return "all" in services or service in services


def normalize_service_token(value, platform="bilibili"):
    aliases = PLATFORM_SERVICE_ALIASES.get(normalize_platform(platform), PLATFORM_SERVICE_ALIASES["bilibili"])
    return aliases.get(str(value or "").strip().lower()) or aliases.get(str(value or "").strip())


def subscription_id_for(platform, uid, target_type, target_id):
    return f"{platform}-{uid}-{target_type}-{target_id}"


def normalize_services(value, platform="bilibili"):
    names = service_names_for(platform)
    source = value if isinstance(value, list) else ["all"]
    result = []
    for item in source:
        service = str(item or "").strip()
        if service in names and service not in result:
            result.append(service)
    if not result or "all" in result:
        return ["all"]
    service_types = service_types_for(platform)
    return ["all"] if all(service in result for service in service_types) else result


def merge_services(existing, incoming, platform="bilibili"):
    names = service_names_for(platform)
    current = [service for service in existing or [] if service in names]
    if "all" in current or "all" in incoming:
        return ["all"]
    result = []
    for service in current + incoming:
        if service in names and service not in result:
            result.append(service)
    return result or ["all"]


def remove_services(existing, removing, platform="bilibili"):
    names = service_names_for(platform)
    current = [service for service in existing or ["all"] if service in names]
    if "all" in removing:
        return []
    if "all" in current:
        current = service_types_for(platform)
    return [service for service in current if service not in removing]


def services_text(services, platform="bilibili"):
    names = service_names_for(platform)
    values = normalize_services(services, platform)
    return "、".join(names.get(service, service) for service in values)


def digits(value):
    text = str(value or "").strip()
    return text if text.isdigit() else ""
