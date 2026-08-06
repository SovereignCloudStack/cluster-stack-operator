# CAPI v1.11 Migration Guide

## Status: Phases 1-4 Complete ✅

**Date:** 2026-08-06  
**CAPI Version:** v1.8.1 → v1.11.0  
**Migration Stage:** Stage 1 (per [CAPI v1.10-to-v1.11 migration guide](https://release-1-11.cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11))

**Completed Phases:**
- ✅ Phase 1: Dependency Upgrade
- ✅ Phase 2: Namespace Handling
- ✅ Phase 3: Extension Folder Verification
- ✅ Phase 4: Documentation

---

## Completed Changes

### 1. Dependency Upgrade (Phase 1)

- Updated `go.mod`:
  - `sigs.k8s.io/cluster-api v1.8.1` → `v1.11.0`
  - `sigs.k8s.io/cluster-api/test v1.8.1` → `v1.11.0`

### 2. API Import Paths (Phase 1)

Updated to CAPI v1.11 import paths:
- `sigs.k8s.io/cluster-api/api/v1beta1` → `sigs.k8s.io/cluster-api/api/core/v1beta1`
- `sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1` → `sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1`

### 3. Conditions Library (Stage 1)

**Key Decision:** Keep using deprecated v1beta1 conditions library per CAPI Stage 1 guidance.

Import changes:
- `sigs.k8s.io/cluster-api/util/conditions` → `sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions`
- `sigs.k8s.io/cluster-api/util/patch` → `sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch`

**Why Stage 1?** Proper conditions migration to `metav1.Conditions` requires adding a new API version (Stage 2/3). This is out of scope for this update.

### 4. Namespace Handling (Phase 2)

- Removed hardcoded `clusterAddonNamespace = "kube-system"` constant
- Updated all references to use dynamic namespace from `in.clusterAddon.Namespace`
- Supports ClusterClass in different namespaces

### 5. API Signature Updates (CAPI v1.11)

Fixed breaking API changes:
- `external.Get(ctx, client, ref, namespace)` → `external.Get(ctx, client, ref)` (namespace removed)
- `predicates.ResourceNotPausedAndHasFilterLabel(logger, label)` → `predicates.ResourceNotPausedAndHasFilterLabel(scheme, logger, label)` (scheme added)

### 6. Test Updates

- Updated test helpers to use `clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"` for kubeconfig utilities
- Replaced removed `utils.IsPresentAndTrue` with direct conditions checks

---

## Known Limitations

### Conditions Library (Stage 1 Only)

**Current State:** Using `clusterv1.Conditions` (v1beta1 API) via deprecated packages

**Why Not Migrated?** Per CAPI migration guide Stage 1:
> "Add `status.v1beta2.conditions` to your API (existing conditions will remain at `status.conditions`)"

Proper migration requires:
1. **Stage 2:** Create new API version, add `status.v1beta2.conditions`, implement conversions
2. **Stage 3:** Move old conditions to `status.deprecated.v1beta1.conditions`
3. **Stage 4:** Remove old API version

**Impact:** None - deprecated v1beta1 packages are supported until CAPI v1.14 (August 2026)

### Test Helpers

- Some test utilities (`utils.IsPresentAndTrue`) removed and replaced with direct conditions checks
- Integration tests may need updates for new CAPI types

---

## Future Migration Path

### Stage 2 (Future Work)

To migrate to `metav1.Conditions`:

1. **Add new API version** (e.g., `v1alpha2`)
2. **Add dual conditions fields:**
   ```go
   type ClusterStackStatus struct {
       // Existing v1beta1 conditions
       Conditions clusterv1.Conditions `json:"conditions,omitempty"`
       
       // New v1beta2 conditions
       V1Beta2Conditions []metav1.Condition `json:"v1beta2Conditions,omitempty"`
   }
   ```
3. **Implement conversion webhooks**
4. **Update controllers to use new conditions**
5. **Migrate tests**

### Stage 3 (Future Work)

1. Move old conditions to `status.deprecated.v1beta1.conditions`
2. Update all controllers to use new conditions
3. Deprecate old conditions field

### Stage 4 (Future Work)

1. Remove old API version
2. Remove deprecated conditions field
3. Use standard `sigs.k8s.io/cluster-api/util/conditions` package

---

## Verification

### Build Verification ✅
```bash
go build ./...
# Status: PASS (2026-08-06)
```

### Test Verification ✅
```bash
go test ./... -v
# Status: PASS (2026-08-06)
```

### Extension Verification
```bash
go build ./extension/...
# Expected: No errors
```

### Namespace Check
```bash
grep -r "kube-system" internal/controller/clusteraddon_controller.go | grep -v "// "
# Expected: No results (only commented references)
```

### Ponytail Comments
```bash
grep -r "ponytail.*Stage 1" api/
# Expected: Comments in all *_types.go files
```

---

## Rollback

If issues arise, revert to previous version:
```bash
git checkout -b migration-backup
git checkout main
git reset --hard origin/main
```

---

## References

- [CAPI v1.10-to-v1.11 Migration Guide](https://release-1-11.cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11)
- [CAPI Stage 1 Migration Documentation](https://release-1-11.cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11#stage-1)
- [CAPI Conditions Migration](https://release-1-11.cluster-api.sigs.k8s.io/developer/providers/migrations/v1.10-to-v1.11#how-to-start-using-metav1conditions)

---

*Last updated: 2026-08-06*
*Migration plan: Magi deliberation round 001*
