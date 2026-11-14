import { cookies } from "next/headers";
import { getRecommendations } from "@/lib/api-server";
import { ANON_COOKIE_NAME } from "@/lib/search-history";
import { RecommendationsView } from "./recommendations-view";

export const dynamic = "force-dynamic";

export const metadata = {
  title: "Recommended for you — GOshop",
  description:
    "Everything GOshop picked for you, plus the crowd favourites when you are new here.",
};

// The service caps a shelf at 50 items, so that is the whole list.
const SHELF_LIMIT = 50;

export default async function RecommendationsPage() {
  const anonId = (await cookies()).get(ANON_COOKIE_NAME)?.value;
  const shelf = await getRecommendations(SHELF_LIMIT, anonId);
  // The body lives in a client component: after a logout on this device the
  // shelf above is still ranked from the retained cookie, and only the browser
  // can read the flag that says to drop it.
  return <RecommendationsView initialShelf={shelf} />;
}
