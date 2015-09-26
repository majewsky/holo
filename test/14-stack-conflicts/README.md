This test checks all the conflicts that can arise during scanning of stacked entities.

* `group:stacked` and `user:stacked` have two maximally conflicting definitions
  in `01-first.json` and `02-second.json`.

And since we're at it anyway, we also check the error case where an entity
definition is missing the required `name` attribute. By placing these broken
groups in `01-first.json` (while the merge conflicts arise in `02-second.json`),
we also check that the entity scanner keeps going and reports all errors at
once.