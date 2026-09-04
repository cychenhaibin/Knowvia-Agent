from __future__ import annotations

import tempfile
import threading
import unittest
from pathlib import Path
from types import SimpleNamespace

from qqa_llm.domain.models import KnowledgeDocument
from qqa_llm.ingest.chunk import ChunkBuilder
from qqa_llm.ingest.normalize import DocumentNormalizer
from qqa_llm.storage.file_repo import FileStore
from qqa_llm.storage.index_job_repo import IndexJobRepository


class BarrierFileStore(FileStore):
    def __init__(self, root: Path) -> None:
        super().__init__(root)
        self.read_barrier = threading.Barrier(2)

    def read_json(self, path: Path, default):
        result = super().read_json(path, default)
        try:
            self.read_barrier.wait(timeout=0.2)
        except threading.BrokenBarrierError:
            pass
        return result


class FileStorageTests(unittest.TestCase):
    def test_failed_json_write_preserves_previous_file(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            store = FileStore(root)
            target = root / "state.json"
            store.write_json(target, {"status": "ready"})

            with self.assertRaises(TypeError):
                store.write_json(target, {"bad": object()})

            self.assertEqual(store.read_json(target, {}), {"status": "ready"})

    def test_concurrent_job_upserts_do_not_lose_updates(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            repo = IndexJobRepository(BarrierFileStore(root), root / "jobs.json")
            threads = [
                threading.Thread(target=repo.upsert, args=("job-1", {"job_id": "job-1"})),
                threading.Thread(target=repo.upsert, args=("job-2", {"job_id": "job-2"})),
            ]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join(timeout=2)

            self.assertIsNotNone(repo.get("job-1"))
            self.assertIsNotNone(repo.get("job-2"))


class ChunkBuilderBoundaryTests(unittest.TestCase):
    def _document(self, body: str) -> KnowledgeDocument:
        return KnowledgeDocument(
            id="doc-1",
            scope_id="scope-1",
            user_id="user-1",
            provider="test",
            external_id="external-1",
            repo="repo",
            title="Title",
            doc_ref="ref",
            source_url="",
            source_updated_at="",
            content_hash="hash",
            version_id="version-1",
            raw_body=body,
            normalized_body=body,
        )

    def test_long_paragraph_never_exceeds_chunk_size(self) -> None:
        builder = ChunkBuilder(SimpleNamespace(chunk_size=10, chunk_overlap=2), DocumentNormalizer())
        chunks = builder.build_chunks(self._document("x" * 25))
        self.assertGreater(len(chunks), 1)
        self.assertTrue(all(0 < len(chunk.content) <= 10 for chunk in chunks), chunks)

    def test_newline_join_is_counted_in_chunk_size(self) -> None:
        builder = ChunkBuilder(SimpleNamespace(chunk_size=10, chunk_overlap=2), DocumentNormalizer())
        chunks = builder.build_chunks(self._document("12345\n67890"))
        self.assertTrue(all(len(chunk.content) <= 10 for chunk in chunks), chunks)

    def test_full_size_paragraph_does_not_retain_overlap(self) -> None:
        builder = ChunkBuilder(SimpleNamespace(chunk_size=10, chunk_overlap=2), DocumentNormalizer())
        chunks = builder.build_chunks(self._document("abc\n1234567890"))
        self.assertTrue(all(len(chunk.content) <= 10 for chunk in chunks), chunks)


if __name__ == "__main__":
    unittest.main()
