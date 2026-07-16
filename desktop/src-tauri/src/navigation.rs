use url::Url;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Destination {
    Home,
    Project(String),
    Workspace(String),
}

impl Destination {
    pub fn route(&self) -> String {
        match self {
            Self::Home => "/".into(),
            Self::Project(id) => format!("/projects/{id}"),
            Self::Workspace(id) => format!("/workspaces?workspace={id}"),
        }
    }
}

pub fn parse_deep_link(value: &str) -> Result<Destination, String> {
    let lower = value.to_ascii_lowercase();
    if value.contains('\\')
        || lower.contains("/../")
        || lower.contains("/./")
        || lower.contains("%2e")
    {
        return Err("deep link contains a navigation segment".into());
    }
    let parsed = Url::parse(value).map_err(|_| "invalid Switchyard deep link".to_owned())?;
    if parsed.scheme() != "switchyard" || parsed.query().is_some() || parsed.fragment().is_some() {
        return Err("unsupported Switchyard deep link".into());
    }
    let kind = parsed
        .host_str()
        .ok_or_else(|| "deep link is missing a resource kind".to_owned())?;
    let segments = parsed
        .path_segments()
        .ok_or_else(|| "deep link has an invalid path".to_owned())?
        .filter(|segment| !segment.is_empty())
        .collect::<Vec<_>>();

    if kind == "home" && segments.is_empty() {
        return Ok(Destination::Home);
    }
    if !matches!(kind, "project" | "workspace") || segments.len() != 1 {
        return Err("deep link route is not supported".into());
    }
    let id = segments[0];
    let valid = !id.is_empty()
        && id.len() <= 128
        && id
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'));
    if !valid {
        return Err("deep link contains an invalid resource identifier".into());
    }
    Ok(match kind {
        "project" => Destination::Project(id.into()),
        _ => Destination::Workspace(id.into()),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_bounded_application_links() {
        assert_eq!(
            parse_deep_link("switchyard://project/abc-123").unwrap(),
            Destination::Project("abc-123".into())
        );
        assert_eq!(
            parse_deep_link("switchyard://workspace/team_one").unwrap(),
            Destination::Workspace("team_one".into())
        );
        assert_eq!(
            Destination::Workspace("team_one".into()).route(),
            "/workspaces?workspace=team_one"
        );
    }

    #[test]
    fn rejects_navigation_and_foreign_origins() {
        assert!(parse_deep_link("https://example.com/project/abc").is_err());
        assert!(parse_deep_link("switchyard://project/../abc").is_err());
        assert!(parse_deep_link("switchyard://project/a?bootstrap=secret").is_err());
    }
}
