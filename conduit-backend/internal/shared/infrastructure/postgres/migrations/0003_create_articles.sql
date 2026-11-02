CREATE TABLE articles (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  author_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slug        VARCHAR(80) NOT NULL,
  title       VARCHAR(100) NOT NULL,
  description VARCHAR(200) NOT NULL,
  body        TEXT NOT NULL,
  seq         BIGINT GENERATED ALWAYS AS IDENTITY,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (slug <> 'feed')
);

CREATE UNIQUE INDEX articles_slug_unique ON articles (slug);
CREATE INDEX articles_author ON articles (author_id);
CREATE INDEX articles_order ON articles (created_at DESC, seq DESC);

CREATE TABLE article_tags (
  article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  position   SMALLINT NOT NULL,
  tag        VARCHAR(30) NOT NULL,
  PRIMARY KEY (article_id, position),
  UNIQUE (article_id, tag)
);

CREATE INDEX article_tags_tag ON article_tags (tag);

CREATE TABLE favorites (
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, article_id)
);

CREATE INDEX favorites_article ON favorites (article_id);
