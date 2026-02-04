#!/usr/bin/env python3
"""
Ingest portfolio project README files into PostgreSQL with pgvector for RAG.

Reads each featured project's README.md, chunks it by ## headers,
generates embeddings via Ollama, and stores them in PostgreSQL with pgvector
for semantic search by the Monty AI chatbot.

Usage:
    python ingest-project-readmes.py                  # Full ingestion
    python ingest-project-readmes.py --dry-run        # Preview without writing
    python ingest-project-readmes.py --clear           # Clear existing data first
    python ingest-project-readmes.py --db-host monty   # Custom DB host
"""

import argparse
import hashlib
import json
import logging
import os
import re
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

import psycopg2
import requests

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

OLLAMA_URL = os.environ.get("OLLAMA_URL", "http://192.168.50.68:11434")
EMBEDDING_MODEL = os.environ.get("EMBEDDING_MODEL", "nomic-embed-text")

DB_HOST = os.environ.get("MONTY_DB_HOST", "192.168.50.68")
DB_PORT = int(os.environ.get("MONTY_DB_PORT", "5432"))
DB_USER = os.environ.get("MONTY_DB_USER", "monty")
DB_PASSWORD = os.environ.get("MONTY_DB_PASSWORD", "monty")
DB_NAME = os.environ.get("MONTY_DB_NAME", "monty")

# Featured projects -- paths are resolved relative to this script's repo root
PROJECTS = [
    {
        "name": "Rayne",
        "github_url": "https://github.com/Nokodoko/rayne",
        "readme_path": os.path.expanduser("~/Portfolio/rayne/README.md"),
        "description": "Go REST API wrapping Datadog for monitoring, webhooks, RUM, and AI-powered RCA",
        "technologies": ["Go", "Datadog", "PostgreSQL", "Docker", "Kubernetes", "Ollama", "Qdrant"],
    },
    {
        "name": "Messages TUI",
        "github_url": "https://github.com/Nokodoko/messages_tui",
        "readme_path": os.path.expanduser("~/Portfolio/messages_tui/README.md"),
        "description": "Terminal UI for Google Messages with vim-like navigation and QR pairing",
        "technologies": ["Go", "Bubble Tea", "Lip Gloss", "TUI"],
    },
    {
        "name": "K8s The Hard Way",
        "github_url": "https://github.com/Nokodoko/k8s_the_hard_way",
        "readme_path": os.path.expanduser("~/Portfolio/k8s_the_hard_way/README.md"),
        "description": "Kubernetes cluster provisioned from scratch on AWS using Terraform",
        "technologies": ["Terraform", "AWS", "Kubernetes"],
    },
    {
        # NOTE: Monty repo is private/upcoming -- use local path only
        "name": "Monty",
        "github_url": "",
        "readme_path": os.path.expanduser("~/monty/README.md"),
        "description": "Local AI agent powered by Ollama with persistent memory and tool-calling",
        "technologies": ["Python", "Ollama", "FastAPI", "DeepSeek-R1", "Semantic Memory"],
    },
]

# Embedding dimension for nomic-embed-text (768-dimensional)
EMBEDDING_DIM = 768

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------


@dataclass
class Chunk:
    """A single chunk of a README file ready for embedding."""

    project_name: str
    github_url: str
    section_title: str
    content: str
    content_hash: str = ""
    metadata: dict = field(default_factory=dict)
    embedding: Optional[list[float]] = None

    def __post_init__(self):
        if not self.content_hash:
            self.content_hash = hashlib.sha256(self.content.encode()).hexdigest()[:16]


# ---------------------------------------------------------------------------
# README chunking
# ---------------------------------------------------------------------------


def read_readme(path: str) -> Optional[str]:
    """Read a README file, returning None if it does not exist."""
    p = Path(path)
    if not p.exists():
        logger.warning("README not found: %s", path)
        return None
    text = p.read_text(encoding="utf-8")
    if not text.strip():
        logger.warning("README is empty: %s", path)
        return None
    return text


def chunk_readme(text: str, project: dict) -> list[Chunk]:
    """
    Split a README into chunks by ## headers.

    Each chunk includes the header line and all content until the next ## header
    (or end of file). The top-level content before the first ## header is kept
    as an "Overview" chunk.
    """
    chunks: list[Chunk] = []
    lines = text.split("\n")

    current_title = "Overview"
    current_lines: list[str] = []

    for line in lines:
        # Detect ## headers (but not ### or deeper)
        header_match = re.match(r"^##\s+(.+)", line)
        if header_match:
            # Flush previous chunk
            if current_lines:
                content = "\n".join(current_lines).strip()
                if content and len(content) > 20:
                    chunks.append(
                        Chunk(
                            project_name=project["name"],
                            github_url=project["github_url"],
                            section_title=current_title,
                            content=content,
                            metadata={
                                "technologies": project.get("technologies", []),
                                "description": project.get("description", ""),
                                "source": "readme",
                            },
                        )
                    )
            current_title = header_match.group(1).strip()
            current_lines = [line]
        else:
            current_lines.append(line)

    # Flush the last chunk
    if current_lines:
        content = "\n".join(current_lines).strip()
        if content and len(content) > 20:
            chunks.append(
                Chunk(
                    project_name=project["name"],
                    github_url=project["github_url"],
                    section_title=current_title,
                    content=content,
                    metadata={
                        "technologies": project.get("technologies", []),
                        "description": project.get("description", ""),
                        "source": "readme",
                    },
                )
            )

    return chunks


# ---------------------------------------------------------------------------
# Ollama embeddings
# ---------------------------------------------------------------------------


def get_embedding(text: str, ollama_url: str = None, model: str = None) -> list[float]:
    """
    Generate an embedding vector for the given text using Ollama's API.

    Raises RuntimeError if the request fails.
    """
    ollama_url = ollama_url or OLLAMA_URL
    model = model or EMBEDDING_MODEL
    url = f"{ollama_url}/api/embed"
    payload = {
        "model": model,
        "input": [text],
    }

    try:
        resp = requests.post(url, json=payload, timeout=60)
        resp.raise_for_status()
    except requests.RequestException as exc:
        raise RuntimeError(f"Ollama embedding request failed: {exc}") from exc

    data = resp.json()
    embeddings = data.get("embeddings")
    if not embeddings or not embeddings[0]:
        raise RuntimeError(f"Ollama returned empty embeddings: {data}")

    return embeddings[0]


# ---------------------------------------------------------------------------
# PostgreSQL / pgvector
# ---------------------------------------------------------------------------

CREATE_EXTENSION_SQL = "CREATE EXTENSION IF NOT EXISTS vector;"

CREATE_TABLE_SQL = """
CREATE TABLE IF NOT EXISTS project_readme_chunks (
    id              SERIAL PRIMARY KEY,
    project_name    TEXT NOT NULL,
    github_url      TEXT NOT NULL,
    section_title   TEXT NOT NULL,
    content         TEXT NOT NULL,
    content_hash    TEXT NOT NULL UNIQUE,
    metadata        JSONB DEFAULT '{}'::jsonb,
    embedding       vector({dim}),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
""".format(dim=EMBEDDING_DIM)

CREATE_INDEX_SQL = """
CREATE INDEX IF NOT EXISTS idx_readme_chunks_embedding
    ON project_readme_chunks
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 10);
"""

UPSERT_SQL = """
INSERT INTO project_readme_chunks
    (project_name, github_url, section_title, content, content_hash, metadata, embedding, updated_at)
VALUES
    (%s, %s, %s, %s, %s, %s, %s, NOW())
ON CONFLICT (content_hash) DO UPDATE SET
    project_name  = EXCLUDED.project_name,
    github_url    = EXCLUDED.github_url,
    section_title = EXCLUDED.section_title,
    content       = EXCLUDED.content,
    metadata      = EXCLUDED.metadata,
    embedding     = EXCLUDED.embedding,
    updated_at    = NOW();
"""

CLEAR_SQL = "DELETE FROM project_readme_chunks;"

COUNT_SQL = "SELECT COUNT(*) FROM project_readme_chunks;"


def get_db_connection(host=None, port=None, user=None, password=None, dbname=None):
    """Create a PostgreSQL connection."""
    host = host or DB_HOST
    port = port or DB_PORT
    user = user or DB_USER
    password = password or DB_PASSWORD
    dbname = dbname or DB_NAME
    try:
        conn = psycopg2.connect(
            host=host,
            port=port,
            user=user,
            password=password,
            dbname=dbname,
        )
        conn.autocommit = False
        return conn
    except psycopg2.Error as exc:
        raise RuntimeError(f"Failed to connect to PostgreSQL at {host}:{port}/{dbname}: {exc}") from exc


def ensure_schema(conn):
    """Create the pgvector extension and table if they do not exist."""
    with conn.cursor() as cur:
        cur.execute(CREATE_EXTENSION_SQL)
        cur.execute(CREATE_TABLE_SQL)
        # Only create the IVFFlat index if we have enough rows (IVFFlat needs data)
        cur.execute(COUNT_SQL)
        count = cur.fetchone()[0]
        if count >= 10:
            try:
                cur.execute(CREATE_INDEX_SQL)
            except psycopg2.Error:
                logger.debug("IVFFlat index creation skipped (may need more rows)")
    conn.commit()


def store_chunk(conn, chunk: Chunk):
    """Upsert a single chunk into the database."""
    embedding_str = "[" + ",".join(str(v) for v in chunk.embedding) + "]" if chunk.embedding else None
    with conn.cursor() as cur:
        cur.execute(
            UPSERT_SQL,
            (
                chunk.project_name,
                chunk.github_url,
                chunk.section_title,
                chunk.content,
                chunk.content_hash,
                json.dumps(chunk.metadata),
                embedding_str,
            ),
        )


def clear_chunks(conn):
    """Remove all existing README chunks."""
    with conn.cursor() as cur:
        cur.execute(CLEAR_SQL)
    conn.commit()
    logger.info("Cleared all existing README chunks from the database")


# ---------------------------------------------------------------------------
# Main ingestion pipeline
# ---------------------------------------------------------------------------


def ingest_project(project: dict, conn, dry_run: bool = False) -> int:
    """
    Ingest a single project's README into the database.

    Returns the number of chunks stored.
    """
    logger.info("Processing project: %s", project["name"])

    readme_text = read_readme(project["readme_path"])
    if readme_text is None:
        logger.warning("Skipping %s -- README not found at %s", project["name"], project["readme_path"])
        return 0

    chunks = chunk_readme(readme_text, project)
    logger.info("  Found %d chunks in %s", len(chunks), project["name"])

    if dry_run:
        for i, chunk in enumerate(chunks, 1):
            logger.info(
                "  [DRY RUN] Chunk %d: section=%r  length=%d chars",
                i,
                chunk.section_title,
                len(chunk.content),
            )
        return len(chunks)

    stored = 0
    for i, chunk in enumerate(chunks, 1):
        try:
            logger.info(
                "  Embedding chunk %d/%d: %s / %s (%d chars)",
                i,
                len(chunks),
                chunk.project_name,
                chunk.section_title,
                len(chunk.content),
            )
            chunk.embedding = get_embedding(chunk.content)
            store_chunk(conn, chunk)
            conn.commit()
            stored += 1
        except RuntimeError as exc:
            logger.error("  Failed to embed/store chunk %d: %s", i, exc)
        except psycopg2.Error as exc:
            logger.error("  Database error storing chunk %d: %s", i, exc)
            conn.rollback()
    logger.info("  Stored %d/%d chunks for %s", stored, len(chunks), project["name"])
    return stored


def main():
    parser = argparse.ArgumentParser(
        description="Ingest portfolio project READMEs into PostgreSQL with pgvector for RAG",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Preview what would be ingested without writing to the database",
    )
    parser.add_argument(
        "--clear",
        action="store_true",
        help="Clear existing README chunks before ingesting",
    )
    parser.add_argument("--db-host", default=DB_HOST, help=f"PostgreSQL host (default: {DB_HOST})")
    parser.add_argument("--db-port", type=int, default=DB_PORT, help=f"PostgreSQL port (default: {DB_PORT})")
    parser.add_argument("--db-user", default=DB_USER, help=f"PostgreSQL user (default: {DB_USER})")
    parser.add_argument("--db-password", default=DB_PASSWORD, help="PostgreSQL password (default: ***)")
    parser.add_argument("--db-name", default=DB_NAME, help=f"PostgreSQL database (default: {DB_NAME})")
    parser.add_argument(
        "--ollama-url", default=OLLAMA_URL, help=f"Ollama API URL (default: {OLLAMA_URL})"
    )
    parser.add_argument(
        "--embedding-model", default=EMBEDDING_MODEL, help=f"Embedding model (default: {EMBEDDING_MODEL})"
    )
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable debug logging")

    args = parser.parse_args()

    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)

    # Override globals from CLI args
    global OLLAMA_URL, EMBEDDING_MODEL, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
    OLLAMA_URL = args.ollama_url
    EMBEDDING_MODEL = args.embedding_model
    DB_HOST = args.db_host
    DB_PORT = args.db_port
    DB_USER = args.db_user
    DB_PASSWORD = args.db_password
    DB_NAME = args.db_name

    if args.dry_run:
        logger.info("=== DRY RUN MODE -- no data will be written ===")
        total = 0
        for project in PROJECTS:
            total += ingest_project(project, conn=None, dry_run=True)
        logger.info("=== DRY RUN COMPLETE: %d total chunks would be ingested ===", total)
        return

    # Connect to PostgreSQL
    logger.info("Connecting to PostgreSQL at %s:%d/%s", DB_HOST, DB_PORT, DB_NAME)
    conn = get_db_connection()
    logger.info("Connected successfully")

    try:
        # Ensure schema exists
        ensure_schema(conn)

        if args.clear:
            clear_chunks(conn)

        # Verify Ollama is reachable
        logger.info("Checking Ollama at %s with model %s", OLLAMA_URL, EMBEDDING_MODEL)
        try:
            test_embedding = get_embedding("test")
            actual_dim = len(test_embedding)
            logger.info("Ollama is reachable, embedding dimension: %d", actual_dim)
            if actual_dim != EMBEDDING_DIM:
                logger.warning(
                    "Expected embedding dimension %d but got %d -- table schema may need updating",
                    EMBEDDING_DIM,
                    actual_dim,
                )
        except RuntimeError as exc:
            logger.error("Cannot reach Ollama: %s", exc)
            sys.exit(1)

        # Ingest all projects
        total = 0
        start = time.time()
        for project in PROJECTS:
            total += ingest_project(project, conn)
        elapsed = time.time() - start

        # Try to create the IVFFlat index now that we have data
        try:
            ensure_schema(conn)
        except Exception:
            pass

        logger.info("=== INGESTION COMPLETE ===")
        logger.info("Total chunks stored: %d", total)
        logger.info("Time elapsed: %.1f seconds", elapsed)

    except Exception:
        logger.exception("Ingestion failed")
        sys.exit(1)
    finally:
        conn.close()


if __name__ == "__main__":
    main()
