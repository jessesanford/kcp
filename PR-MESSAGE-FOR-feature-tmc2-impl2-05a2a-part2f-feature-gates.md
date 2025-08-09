<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR adds comprehensive TMC feature gate configuration and management framework. It introduces the TMCAPIs feature flag that controls all TMC functionality, along with proper version-based rollout mechanisms and user-specific controls.

**Key Components:**
- **TMCAPIs Feature Flag**: Master switch for all TMC functionality (@jessesanford, v0.1)
- **Feature Gate Framework**: Enhanced KCP feature management with version controls
- **Configuration Management**: Proper alpha-stage feature rollout mechanisms
- **Integration Points**: Seamless integration with existing KCP feature infrastructure

**Technical Details:**
- Implements proper KCP feature gate patterns
- Supports version-based feature activation (v1.32+)
- Includes user-based feature flags for controlled rollout
- Maintains backward compatibility with existing feature gates

This is part of the TMC implementation split from an oversized PR (4079 lines → 151 lines).

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5A2A (Feature Gates Infrastructure)

## Release Notes

```
Add TMC feature gates and configuration framework for transparent multi-cluster APIs
```