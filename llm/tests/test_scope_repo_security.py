from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from qqa_llm.storage.file_repo import FileStore
from qqa_llm.storage.scope_repo import ScopeRepository


class ScopeRepositorySecurityTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.base_dir = self.root / "knowledge"
        self.repo = ScopeRepository(FileStore(self.root), self.base_dir)

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_rejects_absolute_and_traversal_identifiers(self) -> None:
        outside = self.root / "outside" / "victim"
        outside.mkdir(parents=True)
        marker = outside / "keep.txt"
        marker.write_text("keep", encoding="utf-8")

        invalid_pairs = [
            (str(self.root / "outside"), "victim"),
            ("..", "outside"),
            ("user", "../outside/victim"),
            ("user/name", "scope"),
            ("user", "scope/name"),
        ]
        for user_id, scope_id in invalid_pairs:
            with self.subTest(user_id=user_id, scope_id=scope_id):
                with self.assertRaises(ValueError):
                    self.repo.delete_scope(user_id, scope_id)

        self.assertEqual(marker.read_text(encoding="utf-8"), "keep")

    def test_rejects_symlink_escape(self) -> None:
        outside = self.root / "outside"
        outside.mkdir()
        (outside / "victim").mkdir()
        (outside / "victim" / "keep.txt").write_text("keep", encoding="utf-8")
        (self.base_dir / "linked-user").symlink_to(outside, target_is_directory=True)

        with self.assertRaises(ValueError):
            self.repo.delete_scope("linked-user", "victim")

        self.assertTrue((outside / "victim" / "keep.txt").exists())


if __name__ == "__main__":
    unittest.main()
