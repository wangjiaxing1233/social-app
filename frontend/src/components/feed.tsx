"use client";

import { useEffect, useState, type FormEvent } from "react";
import { ApiError, deletePost, searchPosts, type Post } from "@/lib/api";
import { EmptyState } from "./empty-state";
import { usernameFromToken } from "@/lib/session";

export function Feed({
  token,
  onExpired,
  revision,
}: {
  token: string;
  revision: number;
  onExpired: () => void;
}) {
  const [filters, setFilters] = useState<{ keywords?: string; user?: string }>({});
  const [posts, setPosts] = useState<Post[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [deleteError, setDeleteError] = useState("");
  const [deletingId, setDeletingId] = useState("");
  const username = usernameFromToken(token);

  async function remove(post: Post) {
    if (deletingId || post.user !== username || !window.confirm("Delete this post?")) return;
    setDeleteError("");
    setDeletingId(post.id);
    try {
      await deletePost(token, post.id);
      setPosts((current) => current.filter(({ id }) => id !== post.id));
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onExpired();
      else setDeleteError("The service could not delete this post.");
    } finally {
      setDeletingId("");
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    searchPosts(token, filters, controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        setPosts(result);
        setLoading(false);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        if (cause instanceof ApiError && cause.status === 401) onExpired();
        else setError("Could not load posts. Search again to retry.");
        setLoading(false);
      });
    return () => controller.abort();
  }, [filters, token, onExpired, revision]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const query = String(data.get("query")).trim();
    setLoading(true);
    setError("");
    setFilters(data.get("scope") === "user" ? { user: query } : { keywords: query });
  }

  return (
    <section className="space-y-4" aria-label="Community posts">
      <form onSubmit={submit}>
        <label>
          Search posts <input name="query" type="search" />
        </label>
        <label>
          Search by
          <select name="scope">
            <option value="keywords">Caption</option>
            <option value="user">Exact creator</option>
          </select>
        </label>
        <button>Search</button>
      </form>

      {loading && <p role="status">Loading posts...</p>}
      {error && <p role="alert">{error}</p>}
      {deleteError && <p role="alert">{deleteError}</p>}
      {!loading && !error && posts.length === 0 && (
        <EmptyState title="No posts found">Try another caption or creator.</EmptyState>
      )}
      {!loading && !error && (
        <div className="grid gap-4 sm:grid-cols-2">
          {posts.map((post) => (
            <article key={post.id} className="panel">
              {post.type === "image" && (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={post.url} alt={post.message || "Community image"} loading="lazy" />
              )}
              {post.type === "video" && <video src={post.url} controls preload="metadata" />}
              {post.type === "unknown" && <p>Media unavailable</p>}
              <h3 className="font-semibold">{post.message}</h3>
              <p>By {post.user}</p>
              {post.user === username && (
                <button disabled={!!deletingId} onClick={() => void remove(post)}>
                  {deletingId === post.id ? "Deleting..." : "Delete"}
                </button>
              )}
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
