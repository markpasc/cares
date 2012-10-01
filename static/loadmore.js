$(function() {
    (function ($) {

        var showingMore = false;
        var oldestItemDate;

        function showMore() {
            if (showingMore) return;
            showingMore = true;

            var $nav = this;
            $nav.find('.load-more').hide();
            $nav.find('.loading').show();

            $.ajax({
                url: '/stream',
                data: { before: oldestItemDate },
                success: function (data) {
                    // put the posts in the page
                    var $posts = $('#posts');
                    var $urpost = $('#editor .post').clone();
                    $urpost.find('.body').attr('contenteditable', 'false');

                    $.each(data, function(i, val) {
                        oldestItemDate = val.Posted;

                        var $post = $urpost.clone();
                        $post.attr('id', 'post-' + val.Id);
                        $post.find('.body').html(val.Html);
                        var posted = new Date(Date.parse(val.Posted));
                        var postedText = $.relatizeDate.strftime(posted, "%i:%M <small>%p</small> %D %b %Y");
                        var $permalink = $post.find('.time a');
                        $permalink.html(postedText);
                        $permalink.attr('href', val.Permalink);

                        $posts.append($post);
                    });

                    $nav.find('.loading').hide();
                    $nav.find('.load-more').show();
                    showingMore = false;
                },
                error: function (xhr) {
                    // show an error
                }
            });
        }

        $.fn.loadMore = function (datestamp) {
            oldestItemDate = datestamp;

            var moreShower = showMore.bind(this);
            this.find('.load-more button').click(moreShower);

            $(window).scroll(function () {
                if ($(window).scrollTop() > $(document).height() - $(window).height()) {
                    moreShower();
                }
            });
        };

    })(jQuery);
});
