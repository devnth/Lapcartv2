package repo

import (
	"context"
	"database/sql"
	"fmt"
	"lapcart/model"
	"lapcart/utils"
	"log"

	"github.com/lib/pq"
)

type ProductRepository interface {
	AddProduct(product model.Product) (string, error)
	FindProductByCode(productCode string) (model.ProductResponse, error)
	UpdateProductColor(newStock int, inStockColor string) error
	UpdateProduct(color model.Color, product model.ProductResponse) error
	GetAllProductCode(pagenation utils.Filter) ([]string, utils.Metadata, error)
	GetAllProductsUser(user_id int, pagenation utils.Filter) ([]model.GetProduct, utils.Metadata, error)
	FindCategory(category string) (int, bool)
	FindBrand(brand string) (int, bool)
	FindProductCode(product_code string) error
	SearchByFilter(filter model.Filter, user_id int, pagenation utils.Filter) ([]model.GetProduct, utils.Metadata, error)
}

type productRepo struct {
	db *sql.DB
}

func NewProductRepo(db *sql.DB) ProductRepository {
	return &productRepo{
		db: db,
	}
}

func (c *productRepo) FindProductByCode(productCode string) (model.ProductResponse, error) {

	ctx := context.Background()
	var product model.ProductResponse

	query1 := `SELECT 
				product.code,
				product.name,
				product.description,
				brand.id,
				brand.name,
				category.id,
				category.name,
				processor.id,
				processor.name,
				product.price,
				product.rating,
				product.image,
				product.is_deleted
				FROM product
				INNER JOIN category ON category.id = product.category_id
				INNER  JOIN brand ON brand.id = product.brand_id
				INNER JOIN processor ON processor.id = product.processor_id
				WHERE product.code = $1;
				`
	query2 := `
				SELECT id, color, stock
				FROM product WHERE code = $1;
	`
	tx, err := c.db.BeginTx(ctx, nil)

	if err != nil {
		return model.ProductResponse{}, err
	}

	err = tx.QueryRow(query1,
		productCode).Scan(
		&product.Code,
		&product.Name,
		&product.Description,
		&product.Brand.ID,
		&product.Brand.Name,
		&product.Category.ID,
		&product.Category.Name,
		&product.Processor.ID,
		&product.Processor.Name,
		&product.Price,
		&product.Rating,
		&product.Image,
		&product.IsDeleted,
	)

	if err != nil {
		return model.ProductResponse{}, err
	}

	rows, err := tx.Query(query2, productCode)

	if err != nil {
		return model.ProductResponse{}, err
	}

	defer rows.Close()

	for rows.Next() {
		var id uint
		var color model.Color

		if err := rows.Scan(&id, &color.Name, &color.Stock); err != nil {
			return product, err
		}

		product.Colors = append(product.Colors, color)
		product.ID = append(product.ID, id)

	}

	if err = rows.Err(); err != nil {
		return product, err
	}

	err = tx.Commit()

	if err != nil {
		return model.ProductResponse{}, err
	}

	return product, nil
}

func (c *productRepo) AddProduct(product model.Product) (string, error) {

	var color string
	var stock int
	ctx := context.Background()

	query3 := `INSERT INTO processor(name)
				VALUES($1)
				RETURNING id;`

	query2 := `INSERT INTO brand(name)
				VALUES($1)
				RETURNING id;`

	query1 := `INSERT INTO category(name)
				VALUES($1)
				RETURNING id;`

	query4 := `
				INSERT INTO product(
				code,
				name,
				color,
				brand_id,
				processor_id,
				category_id,
				price,
				stock)
				VALUES($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING code;`

	// First You begin a transaction with a call to db.Begin()
	tx, err := c.db.BeginTx(ctx, nil)

	if err != nil {
		return "", err
	}

	id, ok := c.FindCategory(product.Category.Name)

	if !ok {
		err = tx.QueryRow(query1,
			product.Category.Name).Scan(&product.Category.ID)
		if err != nil {
			// Incase we find any error in the query execution, rollback the transaction
			tx.Rollback()
			return "", err
		}
	}
	if ok {
		product.Category.ID = uint(id)
	}

	id, ok = c.FindBrand(product.Brand.Name)
	if !ok {
		err = tx.QueryRow(query2,
			product.Brand.Name).Scan(&product.Brand.ID)
		if err != nil {
			// Incase we find any error in the query execution, rollback the transaction
			tx.Rollback()
			return "", err
		}
	}
	if ok {
		product.Brand.ID = uint(id)
	}

	id, ok = c.FindProcessor(product.Processor.Name)
	if !ok {
		err = tx.QueryRow(query3,
			product.Processor.Name).Scan(&product.Processor.ID)
		if err != nil {
			// Incase we find any error in the query execution, rollback the transaction
			tx.Rollback()
			return "", err
		}
	}
	if ok {
		product.Processor.ID = uint(id)
	}

	for _, v := range product.Colors {
		color = v.Name
		stock = v.Stock

		err = tx.QueryRow(query4,
			product.Code,
			product.Name,
			color,
			product.Brand.ID,
			product.Processor.ID,
			product.Category.ID,
			product.Price,
			stock).Scan(
			&product.Code,
		)

		if err != nil {
			// Incase we find any error in the query execution, rollback the transaction
			tx.Rollback()
			return "", err
		}
	}

	err = tx.Commit()
	if err != nil {
		return "", err
	}

	return product.Code, nil

}

func (c *productRepo) UpdateProductColor(newStock int, inStockColor string) error {

	query := `UPDATE product 
				SET 
				stock = (stock + $1) 
				WHERE color = $2;`

	err := c.db.QueryRow(query, newStock, inStockColor).Err()

	if err != nil {
		return err
	}

	return nil
}

func (c *productRepo) UpdateProduct(color model.Color, product model.ProductResponse) error {

	query := `INSERT INTO product
				(code, 
				name,
				color,
				brand_id,
				processor_id,
				category_id,
				price,
				stock)
				VALUES($1, $2, $3, $4, $5, $6, $7, $8);
				`

	err := c.db.QueryRow(
		query,
		product.Code,
		product.Name,
		color.Name,
		product.Brand.ID,
		product.Processor.ID,
		product.Category.ID,
		product.Price,
		color.Stock,
	).Err()

	return err
}

func (c *productRepo) FindCategory(category string) (int, bool) {

	var id int

	query := `SELECT id FROM category WHERE name = $1;`

	err := c.db.QueryRow(query, category).Scan(&id)

	if err == sql.ErrNoRows {
		return 0, false
	}
	return id, true
}

func (c *productRepo) FindBrand(brand string) (int, bool) {

	var id int

	query := `SELECT id FROM brand WHERE name = $1;`

	err := c.db.QueryRow(query, brand).Scan(&id)

	if err == sql.ErrNoRows {
		return 0, false
	}
	return id, true
}

func (c *productRepo) FindProcessor(processor string) (int, bool) {

	var id int

	query := `SELECT id FROM processor WHERE name = $1;`

	err := c.db.QueryRow(query, processor).Scan(&id)

	if err == sql.ErrNoRows {
		return 0, false
	}
	return id, true
}

func (c *productRepo) GetAllProductCode(pagenation utils.Filter) ([]string, utils.Metadata, error) {

	var codes []string
	var totalRecords int

	query := `WITH cte AS (
		SELECT DISTINCT code FROM product)
		SELECT COUNT(*) OVER(), code  FROM cte
		LIMIT $1 OFFSET $2;`

	rows, err := c.db.Query(
		query,
		pagenation.Limit(),
		pagenation.Offset())

	if err != nil {
		return nil, utils.Metadata{}, err
	}

	defer rows.Close()

	for rows.Next() {
		var code string

		if err = rows.Scan(&totalRecords, &code); err != nil {
			return codes, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), err
		}
		codes = append(codes, code)
	}

	if err := rows.Err(); err != nil {
		return codes, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), err
	}

	return codes, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), nil

}

func (c *productRepo) GetAllProductsUser(user_id int, pagenation utils.Filter) ([]model.GetProduct, utils.Metadata, error) {

	var totalRecords int

	query := `WITH wishlist AS 
			  (
			     SELECT
			  	  true as wishlist,
			  	  w.product_code 
			     FROM
			  	  wishlist w 
			     WHERE
			  	  w.user_id = $1
			  )
			  ,
			  discount AS 
			  (
			     SELECT
			  	  d.id,
			  	  d.name,
			  	  d.percentage,
			  	  d.valid_till 
			     FROM
			  	  discount d 
			     WHERE
			  	  status = true 
			  	  AND valid_till > NOW() 
			  )
			  ,
			  product AS 
			  (
			     SELECT
			  	  ARRAY_AGG(p.id) AS id,
			  	  p.code,
			  	  p.name,
			  	  p.category_id,
			  	  p.brand_id,
			  	  p.processor_id,
			  	  p.image,
			  	  p.price,
			  	  w.wishlist,
			  	  p.discount_id,
			  	  ARRAY_AGG(color) AS colors 
			     FROM
			  	  product p 
			  	  LEFT JOIN
			  		 wishlist w 
			  		 ON p.code = w.product_code 
			     GROUP BY
			  	  p.code,
			  	  p.name,
			  	  p.category_id,
			  	  p.brand_id,
			  	  p.processor_id,
			  	  p.discount_id,
			  	  p.image,
			  	  p.price,
			  	  w.wishlist
			  )
			  SELECT
			     COUNT(*) OVER(),
			     p.id,
				 p.code,
			     p.name,
			     c.name,
			     b.name,
			     pr.name,
			     p.image,
			     p.price,
			     COALESCE(p.wishlist, false),
			     p.colors,
			     COALESCE(d.name, ''),
			     COALESCE(cast((p.price * (1 - d.percentage / 100)) AS NUMERIC(10,2)),0) AS discount_price 
			  FROM
			     product p 
			     JOIN
			  	  category c 
			  	  ON p.category_id = c.id 
			     JOIN
			  	  brand b 
			  	  ON p.brand_id = b.id 
			     JOIN
			  	  processor pr 
			  	  ON p.processor_ID = pr.id 
			     LEFT JOIN
			  	  discount d 
			  	  ON p.discount_id = d.id 
			  ORDER BY
			     p.name 
				 LIMIT $2 OFFSET $3;`

	rows, err := c.db.Query(
		query,
		user_id,
		pagenation.Limit(),
		pagenation.Offset())

	if err != nil {
		return nil, utils.Metadata{}, err
	}

	defer rows.Close()
	var products []model.GetProduct

	for rows.Next() {
		var product model.GetProduct

		err = rows.Scan(
			&totalRecords,
			pq.Array(&product.ID),
			&product.Code,
			&product.Name,
			&product.GetCategory.Name,
			&product.GetBrand.Name,
			&product.GetProcessor.Name,
			&product.Image,
			&product.Price,
			&product.WishList,
			pq.Array(&product.GetColor.Name),
			&product.DiscountName,
			&product.DiscountPrice,
		)

		if err != nil {
			return products, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return products, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), err
	}

	return products, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), nil

}

func (c *productRepo) FindProductCode(product_code string) error {

	query := `SELECT
			 	code 
			  FROM
			 	product 
			  WHERE
			 	code = $1;`

	err := c.db.QueryRow(query, product_code).Scan(&product_code)

	return err

}

func (c *productRepo) SearchByFilter(filter model.Filter, user_id int, pagenation utils.Filter) ([]model.GetProduct, utils.Metadata, error) {

	query := `WITH wishlist AS 
			  (
			     SELECT
			  	  true as wishlist,
			  	  w.product_code 
			     FROM
			  	  wishlist w 
			     WHERE
			  	  w.user_id = $1
			  )
			  ,
			  discount AS 
			  (
			     SELECT
			  	  d.id,
			  	  d.name,
			  	  d.percentage,
			  	  d.valid_till 
			     FROM
			  	  discount d 
			     WHERE
			  	  status = true 
			  	  AND valid_till > NOW() 
			  )
			  ,
			  product AS 
			  (
			     SELECT
			  	  ARRAY_AGG(p.id) AS id,
			  	  p.code,
			  	  p.name,
			  	  p.category_id,
			  	  p.brand_id,
			  	  p.processor_id,
			  	  p.image,
			  	  p.price,
			  	  w.wishlist,
				  p.color,
			  	  p.discount_id,
			  	  ARRAY_AGG(color) AS colors 
			     FROM
			  	  product p 
			  	  LEFT JOIN
			  		 wishlist w 
			  		 ON p.code = w.product_code 
			     GROUP BY
			  	  p.code,
			  	  p.name,
			  	  p.category_id,
			  	  p.brand_id,
			  	  p.processor_id,
			  	  p.discount_id,
			  	  p.image,
			  	  p.price,
				  p.color,
			  	  w.wishlist
			  )
			  SELECT
			     COUNT(*) OVER(),
			     p.id,
				 p.code,
			     p.name,
			     c.name,
			     b.name,
			     pr.name,
			     p.image,
			     p.price,
			     COALESCE(p.wishlist, false),
			     p.colors,
			     COALESCE(d.name, ''),
			     COALESCE(cast((p.price * (1 - d.percentage / 100)) AS NUMERIC(10,2)),0) AS discount_price 
			  FROM
			     product p 
			     JOIN
			  	  category c 
			  	  ON p.category_id = c.id 
			     JOIN
			  	  brand b 
			  	  ON p.brand_id = b.id 
			     JOIN
			  	  processor pr 
			  	  ON p.processor_ID = pr.id 
			     LEFT JOIN
			  	  discount d 
			  	  ON p.discount_id = d.id 
				 WHERE `

	var totalRecords int

	i := 2
	var arg []interface{}

	arg = append(arg, user_id)

	if len(filter.Category) != 0 {

		query = query + `c.name IN (`

		for j, category := range filter.Category {
			query = query + "$" + fmt.Sprintf("%d", i)
			if j != len(filter.Category)-1 {
				query = query + ","
			}
			arg = append(arg, category.Name)
			i++
		}
		query = query + ")"
	}

	if len(filter.Brand) != 0 {

		if i > 2 {
			query = query + `AND b.name IN (`
		} else {
			query = query + `b.name IN (`
		}

		for j, brand := range filter.Brand {
			query = query + "$" + fmt.Sprintf("%d", i)
			if j != len(filter.Brand)-1 {
				query = query + ","
			}
			arg = append(arg, brand.Name)
			i++
		}
		query = query + ")"
	}

	if len(filter.Color) != 0 {

		if i > 2 {
			query = query + `AND p.color IN (`
		} else {
			query = query + `p.color IN (`
		}

		for j, color := range filter.Color {
			query = query + "$" + fmt.Sprintf("%d", i)
			if j != len(filter.Color)-1 {
				query = query + ","
			}
			arg = append(arg, color.Name)
			i++
		}
		query = query + ")"
	}

	if len(filter.Processor) != 0 {

		if i > 2 {
			query = query + `AND pr.name IN (`
		} else {
			query = query + `pr.name IN (`
		}

		for j, processor := range filter.Processor {
			query = query + "$" + fmt.Sprintf("%d", i)
			if j != len(filter.Processor)-1 {
				query = query + ","
			}
			arg = append(arg, processor.Name)
			i++
		}
		query = query + ")"
	}

	if len(filter.Name) != 0 {

		if i > 2 {
			query = query + `AND p.name IN (`
		} else {
			query = query + `p.name IN (`
		}

		for j, name := range filter.Name {
			query = query + "$" + fmt.Sprintf("%d", i)
			if j != len(filter.Name)-1 {
				query = query + ","
			}
			arg = append(arg, name.Name)
			i++
		}
		query = query + ")"
	}

	if len(filter.ProductCode) != 0 {

		if i > 2 {
			query = query + `AND p.code IN (`
		} else {
			query = query + `p.code IN (`
		}

		for j, code := range filter.ProductCode {
			query = query + "$" + fmt.Sprintf("%d", i)
			if j != len(filter.ProductCode)-1 {
				query = query + ","
			}
			arg = append(arg, code.ProductCode)
			i++
		}
		query = query + ")"
	}

	query = query + `
					 ORDER BY
					 p.name 
					;`

	// LIMIT $2 OFFSET $3
	log.Println(query)

	stmt, err := c.db.Prepare(query)
	if err != nil {
		log.Println("Error", err)
		log.Println("Error", "Query prepare failed")
		return nil, utils.Metadata{}, err
	}

	res, err := stmt.Query(arg...)
	if err != nil {
		fmt.Println("Error", err)
		fmt.Println("Error", "Query Exec failed")
		return nil, utils.Metadata{}, err
	}

	if err != nil {
		return nil, utils.Metadata{}, err
	}

	defer res.Close()
	var products []model.GetProduct

	for res.Next() {
		var product model.GetProduct

		err = res.Scan(
			&totalRecords,
			pq.Array(&product.ID),
			&product.Code,
			&product.Name,
			&product.GetCategory.Name,
			&product.GetBrand.Name,
			&product.GetProcessor.Name,
			&product.Image,
			&product.Price,
			&product.WishList,
			pq.Array(&product.GetColor.Name),
			&product.DiscountName,
			&product.DiscountPrice,
		)

		if err != nil {
			return products, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), err
		}
		products = append(products, product)
	}

	if err := res.Err(); err != nil {
		return products, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), err
	}

	return products, utils.ComputeMetaData(totalRecords, pagenation.Page, pagenation.PageSize), nil

}
